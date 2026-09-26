COMPOSE = docker compose -f api/deployment/local/docker-compose.yaml

.PHONY: up down test test-api test-web run-api run-web

up: ## Start MongoDB and wait until it is healthy
	$(COMPOSE) up -d --wait

down: ## Stop MongoDB (data is kept in a volume)
	$(COMPOSE) down

test: up test-api test-web ## Run every test. A change is done only when this passes.

test-api:
	cd api && go vet ./... && go test -race -count=1 ./...

test-web: web/node_modules
	cd web && npm run typecheck && npm test

web/node_modules: web/package-lock.json
	cd web && npm ci --no-audit --no-fund
	touch web/node_modules

run-api: up ## Run the API on :8080
	cd api && go run .

run-web: web/node_modules ## Run the web app on :3100 (needs run-api in another terminal)
	cd web && npm run dev
