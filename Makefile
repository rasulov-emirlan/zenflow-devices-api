.PHONY: run test tidy build up down

run:
	go run ./cmd/api

test:
	go test ./...

tidy:
	go mod tidy

build:
	go build -o bin/api ./cmd/api

up:
	docker compose up --build

down:
	docker compose down -v
