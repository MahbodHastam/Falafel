SHELL := /bin/bash

GO ?= go
PNPM ?= pnpm
BIN := bin/falafel
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.DEFAULT_GOAL := help

.PHONY: help install web build-go build dev dev-api dev-web test test-go test-web vet fmt fmt-check typecheck check e2e docker clean

help: ## Show the available targets
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: ## Install web dependencies
	cd web && $(PNPM) install

web: ## Build the Vue client into internal/web/dist
	cd web && $(PNPM) build
	@touch internal/web/dist/.gitkeep

build-go: ## Compile the server binary
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o $(BIN) ./cmd/falafel

build: web build-go ## Build the production binary with the UI embedded

dev: ## Run the Go API and the Vite dev server together
	@trap 'kill 0' INT TERM EXIT; \
	($(GO) run ./cmd/falafel) & \
	(cd web && $(PNPM) dev) & \
	wait

dev-api: ## Run the Go API on :8080
	$(GO) run ./cmd/falafel

dev-web: ## Run the Vite dev server on :5173
	cd web && $(PNPM) dev

test: test-go test-web ## Run all unit tests

test-go: ## Run Go tests
	$(GO) test ./...

test-web: ## Run frontend unit tests
	cd web && $(PNPM) test

vet: ## Run go vet
	$(GO) vet ./...

fmt: ## Format Go sources
	gofmt -w cmd internal

fmt-check: ## Fail if any Go source is not gofmt-formatted
	@test -z "$$(gofmt -l cmd internal)" || { gofmt -l cmd internal; echo "run 'make fmt'"; exit 1; }

typecheck: ## Type-check the web client
	cd web && $(PNPM) typecheck

check: fmt-check vet typecheck test ## Everything CI runs except e2e

e2e: build ## Run Playwright end-to-end tests
	cd web && $(PNPM) e2e

docker: ## Build the Docker image
	docker build --build-arg VERSION=$(VERSION) -t falafel:latest .

clean: ## Remove build artifacts and dependencies
	rm -rf bin web/test-results web/playwright-report web/node_modules internal/web/dist
	mkdir -p internal/web/dist
	touch internal/web/dist/.gitkeep
