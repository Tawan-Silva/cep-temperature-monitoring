# Monitoramento de Temperatura por CEP com OpenTelemetry

Este repositório contém dois microserviços desenvolvidos em Go:

- **Serviço A**: Recebe um CEP através de uma requisição POST, realiza a validação do formato e encaminha o CEP para o Serviço B.
- **Serviço B**: Processa um CEP válido, consulta a localização via serviço ViaCEP, e obtém a temperatura atual através da WeatherAPI. A resposta inclui a temperatura em Celsius, Fahrenheit e Kelvin, juntamente com o nome da cidade correspondente.

## Passos para Executar o Projeto

1. **Construir as Imagens Docker**: No diretório raiz do projeto, utilize o seguinte comando para construir as imagens necessárias:

   ```bash
   docker-compose build
    ```
   
2. **Inicializar os Serviços com Docker Compose**: No diretório raiz (onde o arquivo docker-compose.yml está localizado), execute o comando abaixo para subir os containers:
    
   ```bash
   docker-compose up
   ```

3. **Testar os Endpoints da Aplicação**: Com os serviços em funcionamento, você pode fazer as seguintes requisições para testá-los:

   - Para o Serviço A:
   
     ```bash
     curl -X POST -H "Content-Type: application/json" -d '{"cep":"06532002"}' http://localhost:8080/cep
     ```
   
   - Para o Serviço B (substitua CEP por um CEP válido de 8 dígitos):
   
     ```bash
     curl -X GET "http://localhost:8081/weather?cep=06532002" -H "accept: application/json"
     ```
     
4. **Visualizar Traces no Zipkin**: Acesse o Zipkin no seu navegador utilizando o seguinte endereço: **http://localhost:9411/zipkin/?lookback=15m&endTs=1716854339005&limit=10**
