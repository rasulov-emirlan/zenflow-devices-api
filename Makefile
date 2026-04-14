.PHONY: run test tidy build up down generate lint-spec \
	migrate-up migrate-down migrate-version migrate-create seed-run \
	observe-up observe-down

MIGRATIONS_DIR := internal/storage/postgresql/migrations

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
	docker compose up --build -d postgres api prometheus grafana tempo otel-collector

down:
	docker compose down -v

observe-up:
	docker compose up -d prometheus grafana tempo otel-collector

observe-down:
	docker compose stop prometheus grafana tempo otel-collector

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	@test -n "$(N)" || (echo "usage: make migrate-down N=1" && exit 2)
	go run ./cmd/migrate down $(N)

migrate-version:
	go run ./cmd/migrate version

# migrate-create NAME=add_xyz -> scaffolds NNNN_add_xyz.{up,down}.sql
migrate-create:
	@test -n "$(NAME)" || (echo "usage: make migrate-create NAME=add_xyz" && exit 2)
	@next=$$(ls $(MIGRATIONS_DIR) 2>/dev/null | grep -Eo '^[0-9]+' | sort -n | tail -1); \
	  next=$$(printf "%04d" $$((10#$${next:-0} + 1))); \
	  up=$(MIGRATIONS_DIR)/$${next}_$(NAME).up.sql; \
	  down=$(MIGRATIONS_DIR)/$${next}_$(NAME).down.sql; \
	  : > $$up && : > $$down && \
	  echo "created $$up" && echo "created $$down"

seed-run:
	go run ./cmd/seed run --source=templates --on-conflict=skip
