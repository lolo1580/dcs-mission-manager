# DCS Mission Manager — build helpers

.PHONY: help frontend backend build run test lint docker clean

help: ## Show this help
	@echo "Targets:"
	@echo "  frontend  Build the web UI (Svelte + Vite)"
	@echo "  backend   Build the Go binary (frontend must be built first)"
	@echo "  build     Build frontend then backend"
	@echo "  run       Run the backend from source"
	@echo "  test      Run Go tests"
	@echo "  docker    Build the Docker image"
	@echo "  clean     Remove build artifacts"

frontend: ## Build the web UI
	cd frontend && npm install && npm run build

backend: ## Build the Go binary
	cd backend && go build -o ../dcsmm ./cmd/dcsmm

build: frontend backend ## Build everything

run: ## Run the backend from source
	cd backend && go run ./cmd/dcsmm

test: ## Run Go tests
	cd backend && go test ./...

docker: ## Build the Docker image
	docker build -f deploy/Dockerfile -t dcsmm:latest .

clean: ## Remove build artifacts
	rm -rf dcsmm dcsmm.exe frontend/dist backend/internal/api/dist/assets backend/internal/api/dist/index.html
