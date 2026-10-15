# Load .env if present, otherwise fall back to .env.example so a fresh clone just works.
ENV_FILE := $(if $(wildcard .env),.env,.env.example)
include $(ENV_FILE)
export

SQLC_VERSION  := v1.31.1
GOOSE_VERSION := v3.28.0
AIR_VERSION   := v1.67.4
NODEMON_VERSION := 3.1.11
REDOCLY_VERSION := 2.64.0

.PHONY: help up down up-app down-app docker-build run dev nodemon build migrate migrate-down migrate-status migrate-create sqlc sqlc-check test test-integration vet tidy lint-api

help: ## Show available targets
	@grep -hE '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-16s %s\n", $$1, $$2}'

up: ## Start Postgres
	docker compose up -d

down: ## Stop Postgres
	docker compose down

APP_COMPOSE := docker compose -f docker-compose.yml -f docker-compose.app.yml

up-app: ## Run both APIs in containers (needs martgo-auth's Postgres up); gateway on :8000
	$(APP_COMPOSE) up -d --build

down-app: ## Stop the API containers and point the gateway back at the host
	$(APP_COMPOSE) rm -sf mart-api mart-migrate auth-api auth-migrate
	docker compose up -d gateway

docker-build: ## Build the API image (martgo-api:dev)
	docker build -t martgo-api:dev .

run: ## Run the API
	go run ./cmd/api

dev: ## Run the API with hot reload (Air)
	go run github.com/air-verse/air@$(AIR_VERSION)

nodemon: ## Run with auto-restart using nodemon (needs Node.js; config in nodemon.json)
	npx --yes nodemon@$(NODEMON_VERSION)

build: ## Build the API binary into ./bin
	go build -o bin/martgo ./cmd/api

migrate: ## Apply all migrations (the admin is seeded by martgo-auth)
	go run ./cmd/migration up

migrate-down: ## Roll back the last migration
	go run ./cmd/migration down

migrate-status: ## Show migration status
	go run ./cmd/migration status

migrate-create: ## New SQL migration: make migrate-create name=add_something
	go run github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir migrations -s create $(name) sql

sqlc: ## Generate type-safe query code into internal/db
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

sqlc-check: ## Fail if internal/db is out of date with the SQL (run make sqlc to fix)
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) diff

test: ## Run unit tests
	go test -race -count=1 ./...

TEST_DATABASE_URL ?= postgres://$(or $(POSTGRES_USER),martgo):$(or $(POSTGRES_PASSWORD),martgo)@localhost:$(or $(POSTGRES_PORT),5433)/martgo_test?sslmode=disable

test-integration: ## Run integration tests against the martgo_test database (wiped on every test)
	TEST_DATABASE_URL='$(TEST_DATABASE_URL)' go test -tags=integration -race -count=1 ./...

vet: ## Run go vet
	go vet ./...

tidy: ## Tidy go.mod
	go mod tidy

lint-api: ## Validate the OpenAPI spec
	npx --yes @redocly/cli@$(REDOCLY_VERSION) lint api/openapi.yaml
