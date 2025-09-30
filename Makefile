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

# Pinned dev tool versions (update as needed for reproducible tooling)
GOLANGCI_LINT_VERSION:=v1.63.4
GOLANGCI_LINT_MODULE:=github.com/golangci/golangci-lint/v2/cmd/golangci-lint
AIR_VERSION:=v1.40.0
AIR_MODULE:=github.com/cosmtrek/air
GOVULNCHECK_VERSION:=v0.1.0
GOVULNCHECK_MODULE:=golang.org/x/vuln/cmd/govulncheck

# Build targets
.PHONY: all build clean test test-coverage test-integration test-load lint docker help frontend

all: clean lint test frontend build ## Build everything

build: ## Build the binary
	$(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/ysf-nexus

frontend: ## Build the frontend
	@echo "Building frontend..."
	cd frontend && npm install && npm run build
	@echo "Frontend built successfully"

build-linux: ## Build for Linux
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux ./cmd/ysf-nexus

build-native: ## Build for native platform (current OS/ARCH)
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-native ./cmd/ysf-nexus

build-windows: ## Build for Windows
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-windows.exe ./cmd/ysf-nexus

build-darwin: ## Build for macOS
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin ./cmd/ysf-nexus

build-all: build-linux build-windows build-darwin ## Build for all platforms

clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -rf bin/
	rm -rf coverage/
	rm -rf pkg/web/dist/*
	cd frontend && rm -rf node_modules/ dist/

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

lint: ## Run linter (golangci-lint)
	@echo "Running golangci-lint..."
	@command -v golangci-lint >/dev/null 2>&1 || (echo "golangci-lint not found. Install with:"; echo "  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.63.4"; exit 1)
	@golangci-lint run ./... \
		--timeout=5m

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

dev-frontend: ## Run frontend in development mode
	cd frontend && npm run dev

dev-full: ## Run both backend and frontend in development mode
	@echo "Starting backend and frontend in development mode"
	make dev & make dev-frontend

install-tools: ## Install development tools (pinned versions)
	@echo "Installing development tools (pinned versions)"
	@echo "golangci-lint: $(GOLANGCI_LINT_VERSION)"
	@echo "air: $(AIR_VERSION)"
	@echo "govulncheck: $(GOVULNCHECK_VERSION)"
	# Use `go install` with explicit module@version for reproducible installs
	$(GOCMD) install $(GOLANGCI_LINT_MODULE)@$(GOLANGCI_LINT_VERSION)
	$(GOCMD) install $(AIR_MODULE)@$(AIR_VERSION)
	$(GOCMD) install $(GOVULNCHECK_MODULE)@$(GOVULNCHECK_VERSION)

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)