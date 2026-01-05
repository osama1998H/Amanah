# Amanah Payment Gateway - Makefile
# Common operations for development and deployment

.PHONY: all build test lint clean docker help

# Variables
BINARY_NAME=amanah
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT_SHA=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-w -s -X main.Version=$(VERSION) -X main.CommitSHA=$(COMMIT_SHA) -X main.BuildTime=$(BUILD_TIME)"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOMOD=$(GOCMD) mod
GOFMT=gofmt

# Docker parameters
DOCKER_IMAGE=amanah/gateway
DOCKER_TAG?=$(VERSION)

# Default target
all: lint test build

# ============================================
# Development
# ============================================

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

## lint: Run linters
lint:
	@echo "Running linters..."
	$(GOVET) ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout=5m; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...

# ============================================
# Build
# ============================================

## build: Build all binaries
build: build-gateway build-api

## build-gateway: Build gateway binary
build-gateway:
	@echo "Building gateway..."
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o bin/gateway ./cmd/gateway

## build-api: Build API binary
build-api:
	@echo "Building API..."
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o bin/api ./cmd/api

## build-linux: Build for Linux
build-linux:
	@echo "Building for Linux..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/gateway-linux ./cmd/gateway
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/api-linux ./cmd/api

## build-darwin: Build for macOS
build-darwin:
	@echo "Building for macOS..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/gateway-darwin ./cmd/gateway
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o bin/gateway-darwin-arm64 ./cmd/gateway

# ============================================
# Testing
# ============================================

## test: Run all tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race ./...

## test-coverage: Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@mkdir -p coverage
	$(GOTEST) -v -race -coverprofile=coverage/coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage/coverage.out -o coverage/coverage.html
	$(GOCMD) tool cover -func=coverage/coverage.out | tee coverage/coverage.txt
	@echo ""
	@echo "Coverage report generated:"
	@echo "  - HTML: coverage/coverage.html"
	@echo "  - Text: coverage/coverage.txt"
	@tail -1 coverage/coverage.txt

## test-short: Run short tests only
test-short:
	@echo "Running short tests..."
	$(GOTEST) -v -short ./...

## test-integration: Run integration tests
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v -tags=integration ./tests/integration/...

## test-e2e: Run end-to-end tests
test-e2e:
	@echo "Running E2E tests..."
	$(GOTEST) -v -tags=e2e ./tests/e2e/...

## test-all: Run all tests (unit, integration, e2e)
test-all: test test-integration test-e2e

## test-service: Run tests for a specific service (usage: make test-service SERVICE=account)
test-service:
	@echo "Running tests for $(SERVICE) service..."
	$(GOTEST) -v -race -coverprofile=coverage/$(SERVICE).out ./services/$(SERVICE)/...
	$(GOCMD) tool cover -func=coverage/$(SERVICE).out

## bench: Run benchmarks
bench:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

# ============================================
# Docker
# ============================================

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT_SHA=$(COMMIT_SHA) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		--target production \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) .

## docker-build-dev: Build development Docker image
docker-build-dev:
	@echo "Building development Docker image..."
	docker build --target development -t $(DOCKER_IMAGE):dev .

## docker-push: Push Docker image
docker-push:
	@echo "Pushing Docker image..."
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

## docker-run: Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run -it --rm -p 8080:8080 -p 9090:9090 $(DOCKER_IMAGE):$(DOCKER_TAG)

# ============================================
# Docker Compose
# ============================================

## up: Start all services
up:
	@echo "Starting services..."
	docker-compose up -d

## down: Stop all services
down:
	@echo "Stopping services..."
	docker-compose down

## logs: View logs
logs:
	docker-compose logs -f

## ps: List running services
ps:
	docker-compose ps

## restart: Restart services
restart: down up

## clean-docker: Remove all containers and volumes
clean-docker:
	@echo "Cleaning Docker resources..."
	docker-compose down -v --remove-orphans

# ============================================
# Database
# ============================================

## migrate-up: Run database migrations
migrate-up:
	@echo "Running migrations..."
	$(GOCMD) run ./cmd/migrate up

## migrate-down: Rollback database migrations
migrate-down:
	@echo "Rolling back migrations..."
	$(GOCMD) run ./cmd/migrate down

## migrate-status: Show migration status
migrate-status:
	@echo "Migration status..."
	$(GOCMD) run ./cmd/migrate status

# ============================================
# Security
# ============================================

## security-scan: Run security scans
security-scan:
	@echo "Running security scans..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec -fmt=json -out=gosec-results.json ./...; \
	else \
		echo "gosec not installed, skipping..."; \
	fi

## vuln-check: Check for vulnerabilities
vuln-check:
	@echo "Checking for vulnerabilities..."
	$(GOCMD) list -json -m all | docker run --rm -i sonatypecommunity/nancy:latest sleuth

# ============================================
# Documentation
# ============================================

## docs: Generate documentation
docs:
	@echo "Generating documentation..."
	$(GOCMD) doc -all ./... > docs/godoc.txt

## swagger: Generate Swagger documentation
swagger:
	@echo "Generating Swagger documentation..."
	@if command -v swag >/dev/null 2>&1; then \
		swag init -g cmd/api/main.go -o docs/swagger; \
	else \
		echo "swag not installed, skipping..."; \
	fi

# ============================================
# Utilities
# ============================================

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -f gosec-results.json

## install-tools: Install development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install github.com/cosmtrek/air@latest

## version: Show version
version:
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT_SHA)"
	@echo "Build Time: $(BUILD_TIME)"

## help: Show this help
help:
	@echo "Amanah Payment Gateway - Available targets:"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed -e 's/## //' | column -t -s ':'
