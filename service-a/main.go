package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"log"
	"net/http"
	"strconv"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type CEPRequest struct {
	CEP string `json:"cep"`
}

func main() {
	initTracer()

	http.HandleFunc("/cep", handleCEP)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleCEP(w http.ResponseWriter, r *http.Request) {
	var req CEPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handleError(w, "error decoding request", http.StatusBadRequest)
		return
	}

	if !isValidCEP(req.CEP) {
		handleError(w, "invalid zipcode", http.StatusUnprocessableEntity)
		return
	}

	ctx := r.Context()
	tracer := otel.Tracer("service-a")
	ctx, span := tracer.Start(ctx, "handleCEP")
	defer span.End()

	weatherResp, err := getWeatherFromServiceB(ctx, req.CEP)
	if err != nil {
		handleError(w, "service B error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(weatherResp); err != nil {
		handleError(w, "error encoding response", http.StatusInternalServerError)
	}
}

func isValidCEP(cep string) bool {
	return len(cep) == 8 && isNumeric(cep)
}

func getWeatherFromServiceB(ctx context.Context, cep string) (map[string]interface{}, error) {
	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
	serviceBUrl := fmt.Sprintf("http://service-b:8081/weather?cep=%s", cep)

	req, _ := http.NewRequestWithContext(ctx, "GET", serviceBUrl, nil)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("service B returned status: %d", resp.StatusCode)
	}

	var weatherResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&weatherResp); err != nil {
		return nil, err
	}

	return weatherResp, nil
}

func handleError(w http.ResponseWriter, message string, statusCode int) {
	http.Error(w, message, statusCode)
}

func isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func initTracer() {
	ctx := context.Background()
	client := otlptracehttp.NewClient(otlptracehttp.WithEndpoint("otel-collector:4317"), otlptracehttp.WithInsecure())
	exporter, err := otlptrace.New(ctx, client)

	if err != nil {
		log.Fatalf("failed to create exporter: %v", err)
	}

	zipkinExporter, err := zipkin.New("http://zipkin:9411/api/v2/spans")
	if err != nil {
		log.Fatalf("failed to create zipkin exporter: %v", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithBatcher(zipkinExporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("service-a"),
		)),
	)
	otel.SetTracerProvider(tp)
}
