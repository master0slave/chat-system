COMPOSE = docker compose -f api/deployment/local/docker-compose.yaml

.PHONY: up down test test-api run-api

up: ## Start MongoDB and wait until it is healthy
	$(COMPOSE) up -d --wait

down: ## Stop MongoDB (data is kept in a volume)
	$(COMPOSE) down

test: up test-api ## Run every test. A change is done only when this passes.

test-api:
	cd api && go vet ./... && go test -race -count=1 ./...

run-api: up ## Run the API on :8080
	cd api && go run .
