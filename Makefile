.PHONY: run test tidy build up down generate lint-spec

generate:
	go tool oapi-codegen -config oapi-codegen.yaml api/openapi.yaml

lint-spec:
	npx --yes @redocly/cli lint api/openapi.yaml

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
