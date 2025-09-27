# YSF Nexus Makefile

# Variables
BINARY_NAME=ysf-nexus
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date +%FT%T%z)
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build targets
.PHONY: all build clean test test-coverage test-integration test-load lint docker help

all: clean lint test build ## Build everything

build: ## Build the binary
	$(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/ysf-nexus

build-linux: ## Build for Linux
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux ./cmd/ysf-nexus

build-windows: ## Build for Windows
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-windows.exe ./cmd/ysf-nexus

build-darwin: ## Build for macOS
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin ./cmd/ysf-nexus

build-all: build-linux build-windows build-darwin ## Build for all platforms

clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -rf bin/
	rm -rf coverage/

test: ## Run unit tests
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage
	mkdir -p coverage
	$(GOTEST) -coverprofile=coverage/coverage.out ./...
	$(GOCMD) tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "Coverage report generated: coverage/coverage.html"

test-integration: ## Run integration tests
	$(GOTEST) -v -tags=integration ./...

test-load: ## Run load tests
	$(GOTEST) -v -tags=load ./...

test-bench: ## Run benchmarks
	$(GOTEST) -bench=. -benchmem ./...

lint: ## Run linter
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found, install with: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.54.2"; \
	fi

fmt: ## Format code
	$(GOCMD) fmt ./...

mod-tidy: ## Tidy go modules
	$(GOMOD) tidy

deps: ## Download dependencies
	$(GOMOD) download

docker: ## Build Docker image
	docker build -t $(BINARY_NAME):$(VERSION) .
	docker tag $(BINARY_NAME):$(VERSION) $(BINARY_NAME):latest

docker-run: ## Run Docker container
	docker run -p 42000:42000/udp -p 8080:8080 $(BINARY_NAME):latest

run: build ## Build and run
	./bin/$(BINARY_NAME)

dev: ## Run in development mode with hot reload
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "air not found, install with: go install github.com/cosmtrek/air@latest"; \
		$(GOBUILD) -o bin/$(BINARY_NAME) ./cmd/ysf-nexus && ./bin/$(BINARY_NAME); \
	fi

install-tools: ## Install development tools
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOGET) github.com/cosmtrek/air@latest

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)