.PHONY: run build tidy clean

## Ejecutar en desarrollo
run:
	go run ./cmd/bot

## Compilar binario
build:
	go build -o bin/keyholder-bot ./cmd/bot

## Instalar dependencias
tidy:
	go mod tidy

## Limpiar binarios
clean:
	rm -rf bin/
