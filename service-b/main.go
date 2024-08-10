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
	"net/url"
	"strconv"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type WeatherResponse struct {
	City  string  `json:"city"`
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

func main() {
	initTracer()

	http.HandleFunc("/weather", handleWeather)
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func handleWeather(w http.ResponseWriter, r *http.Request) {
	cep := r.URL.Query().Get("cep")
	if !isValidCEP(cep) {
		handleError(w, "invalid zipcode", http.StatusUnprocessableEntity)
		return
	}

	ctx := r.Context()
	tracer := otel.Tracer("service-b")
	ctx, span := tracer.Start(ctx, "handleWeather")
	defer span.End()

	location, err := getLocation(ctx, cep)
	if err != nil {
		handleError(w, "can not find zipcode", http.StatusNotFound)
		return
	}

	weather, err := getWeather(ctx, location)
	if err != nil {
		handleError(w, "error fetching weather", http.StatusInternalServerError)
		return
	}

	response := WeatherResponse{
		City:  location,
		TempC: weather,
		TempF: weather*1.8 + 32,
		TempK: weather + 273,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		handleError(w, "error encoding response", http.StatusInternalServerError)
	}
}

func isValidCEP(cep string) bool {
	return len(cep) == 8 && isNumeric(cep)
}

func getLocation(ctx context.Context, cep string) (string, error) {
	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
	url := fmt.Sprintf("https://viacep.com.br/ws/%s/json/", cep)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get location")
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	location, ok := data["localidade"].(string)
	if !ok {
		return "", fmt.Errorf("localidade not found")
	}

	return location, nil
}

func getWeather(ctx context.Context, location string) (float64, error) {
	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	escapedLocation := url.QueryEscape(location)
	url := fmt.Sprintf("http://api.weatherapi.com/v1/current.json?key=e5bd00e528e346ff8a840254213009&q&q=%s", escapedLocation)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("error creating weather request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("error executing weather request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to get weather, status code: %d", resp.StatusCode)
	}

	var result struct {
		Current struct {
			TempC float64 `json:"temp_c"`
		} `json:"current"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("error parsing weather response: %v", err)
	}

	return result.Current.TempC, nil
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
	client := otlptracehttp.NewClient(
		otlptracehttp.WithEndpoint("otel-collector:4317"),
		otlptracehttp.WithInsecure(),
	)
	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		log.Fatalf("failed to create exporter: %v", err)
	}

	zipkinExporter, err := zipkin.New("http://zipkin:9411/api/v2/spans")
	if err != nil {
		log.Fatalf("failed to create zipkin exporter: %v", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithBatcher(zipkinExporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("service-b"),
		)),
	)
	otel.SetTracerProvider(tp)
}
