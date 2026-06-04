.PHONY: help build run test test-short test-cover test-race fmt clean migrate-up migrate-down migrate-reset migrate-version docker-up docker-down docker-rebuild docker-logs docker-logs-bank docker-ps docker-restart smoke-authorize smoke-declined smoke-expired health

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the gateway binary
	@go build -o bin/gateway ./cmd/gateway

run: ## Run the gateway locally
	@go run ./cmd/gateway

fmt: ## Format all Go code
	@gofmt -w .

clean: ## Remove build artifacts
	@rm -rf bin/ tmp/

test: ## Run all tests
	@go test -v -count=1 ./...

test-short: ## Run unit tests only
	@go test -v -short ./...

test-cover: ## Run tests with coverage report
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html

test-race: ## Run tests with race detector
	@go test -v -race ./...

migrate-up: ## Run all pending migrations
	@migrate -path db/migrations -database "$(DATABASE_URL)" up

migrate-down: ## Roll back the last migration
	@migrate -path db/migrations -database "$(DATABASE_URL)" down 1

migrate-reset: ## Roll back ALL migrations
	@migrate -path db/migrations -database "$(DATABASE_URL)" down

migrate-version: ## Show current migration version
	@migrate -path db/migrations -database "$(DATABASE_URL)" version

docker-up: ## Start all containers
	@cd ../docker && docker compose up -d

docker-down: ## Stop all containers
	@cd ../docker && docker compose down

docker-rebuild: ## Rebuild and restart all containers
	@cd ../docker && docker compose up --build -d

docker-logs: ## Tail gateway logs
	@cd ../docker && docker compose logs -f gateway

docker-logs-bank: ## Tail bank API logs
	@cd ../docker && docker compose logs -f bank-api

docker-ps: ## Show container status
	@cd ../docker && docker compose ps

docker-restart: ## Restart gateway container only
	@cd ../docker && docker compose restart gateway

smoke-authorize: ## Test happy path authorize
	@curl -s -X POST http://localhost:8088/payments/authorize -H "Content-Type: application/json" -H "Idempotency-Key: smoke-test-1" -d "{\"order_id\":\"smoke-order-1\",\"customer_id\":\"smoke-customer-1\",\"amount\":5000,\"currency\":\"USD\",\"card_number\":\"4111111111111111\",\"card_expiry\":\"12/2030\",\"card_cvv\":\"123\"}"

smoke-declined: ## Test declined card
	@curl -s -X POST http://localhost:8088/payments/authorize -H "Content-Type: application/json" -H "Idempotency-Key: smoke-declined-1" -d "{\"order_id\":\"smoke-order-2\",\"customer_id\":\"smoke-customer-2\",\"amount\":5000,\"currency\":\"USD\",\"card_number\":\"5555555555554444\",\"card_expiry\":\"06/2030\",\"card_cvv\":\"456\"}"

smoke-expired: ## Test expired card
	@curl -s -X POST http://localhost:8088/payments/authorize -H "Content-Type: application/json" -H "Idempotency-Key: smoke-expired-1" -d "{\"order_id\":\"smoke-order-3\",\"customer_id\":\"smoke-customer-3\",\"amount\":5000,\"currency\":\"USD\",\"card_number\":\"5105105105105100\",\"card_expiry\":\"03/2020\",\"card_cvv\":\"321\"}"

health: ## Check gateway health
	@curl -s http://localhost:8088/health