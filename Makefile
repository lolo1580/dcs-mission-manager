# DCS Mission Manager — build helpers

VERSION := $(shell cat VERSION 2>/dev/null || echo dev)

.PHONY: help frontend backend build run install test clean

help: ## Show this help
	@echo "Targets:"
	@echo "  frontend        Build the web UI (Svelte + Vite)"
	@echo "  backend         Build the Go binary (frontend must be built first)"
	@echo "  build           Build frontend then backend"
	@echo "  run             Run the backend from source"
	@echo "  install         Build everything then install the Lua scripts into DCS"
	@echo "  test            Run Go tests"
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

clean: ## Remove build artifacts
	rm -rf dcsmm dcsmm.exe frontend/dist backend/internal/api/dist/assets backend/internal/api/dist/index.html
