# DCS Mission Manager — build helpers

VERSION := $(shell cat VERSION 2>/dev/null || echo dev)

.PHONY: help frontend backend build run install test docker docker-multiarch clean

help: ## Show this help
	@echo "Targets:"
	@echo "  frontend        Build the web UI (Svelte + Vite)"
	@echo "  backend         Build the Go binary (frontend must be built first)"
	@echo "  build           Build frontend then backend"
	@echo "  run             Run the backend from source"
	@echo "  install         Build everything then install the Lua scripts into DCS"
	@echo "  test            Run Go tests"
	@echo "  docker          Build the Docker image"
	@echo "  docker-multiarch Build for linux/amd64 and linux/arm64"
	@echo "  clean           Remove build artifacts"

frontend: ## Build the web UI
	cd frontend && npm install && npm run build

backend: ## Build the Go binary
	cd backend && go build -trimpath -ldflags="-s -w -X main.Version=$(VERSION)" -o ../dcsmm ./cmd/dcsmm

build: frontend backend ## Build everything

run: ## Run the backend from source
	cd backend && go run ./cmd/dcsmm

install: build ## Build then install the Lua scripts into DCS
	./dcsmm install-lua

test: ## Run Go tests
	cd backend && go test ./...

docker: ## Build the Docker image
	docker build --build-arg VERSION=$(VERSION) -f deploy/Dockerfile -t dcsmm:$(VERSION) -t dcsmm:latest .

docker-multiarch: ## Build for linux/amd64 and linux/arm64
	docker buildx build --platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) -f deploy/Dockerfile -t dcsmm:$(VERSION) .

clean: ## Remove build artifacts
	rm -rf dcsmm dcsmm.exe frontend/dist backend/internal/api/dist/assets backend/internal/api/dist/index.html
