# Makefile for kimi-go

# Variables
BINARY_NAME=kimi
VERSION=0.1.0
BUILD_DIR=build
INSTALL_PREFIX=/usr/local

# Ensure build directory exists
$(shell mkdir -p $(BUILD_DIR))

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -s -w"

# Default target
.PHONY: all build test clean install uninstall fmt vet lint check run help benchmark verify

all: check build

## help: Show this help message
help:
	@echo "Available targets:"
	@grep -E '^## [a-zA-Z_-]+:.*$$' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
	@echo ""
	@echo "Variables:"
	@echo "  VERSION=$(VERSION)"
	@echo "  BUILD_DIR=$(BUILD_DIR)"
	@echo "  INSTALL_PREFIX=$(INSTALL_PREFIX)"

## build: Build the binary to build directory
build:
	@echo "Building $(BINARY_NAME) v$(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/kimi
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)"

## test: Run all tests with coverage
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@echo ""
	@echo "Coverage report:"
	@go tool cover -func=coverage.out | tail -5

## test-short: Run tests without race detector (faster)
test-short:
	@echo "Running tests (short mode)..."
	$(GOTEST) -coverprofile=coverage.out ./...

## clean: Remove build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out
	@rm -f test_output.txt
	@rm -f kimi_test
	@echo "Clean complete."

## install: Install binary to system (requires sudo)
install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_PREFIX)/bin..."
	@install -d $(INSTALL_PREFIX)/bin
	@install -m 755 $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_PREFIX)/bin/
	@echo "Installed: $(INSTALL_PREFIX)/bin/$(BINARY_NAME)"
	@echo ""
	@echo "Verify installation:"
	@which $(BINARY_NAME) || echo "Not in PATH"
	@$(INSTALL_PREFIX)/bin/$(BINARY_NAME) -version

## uninstall: Remove binary from system (requires sudo)
uninstall:
	@echo "Uninstalling $(BINARY_NAME) from $(INSTALL_PREFIX)/bin..."
	@rm -f $(INSTALL_PREFIX)/bin/$(BINARY_NAME)
	@echo "Uninstall complete."

## fmt: Format Go code
fmt:
	@echo "Formatting Go code..."
	@gofmt -s -w .
	@echo "Format complete."

## vet: Run go vet
vet:
	@echo "Running go vet..."
	@go vet ./...
	@echo "Vet complete."

## lint: Run linter (if installed)
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "Running golangci-lint..."; \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

## check: Run all checks (fmt, vet, test)
check: fmt vet test-short
	@echo "All checks passed!"

## test-integration: Run integration tests (requires API key)
# Usage: make test-integration OPENAI_API_KEY=your-key OPENAI_MODEL=your-model
test-integration:
	@echo "Running integration tests..."
	@if [ -z "$(OPENAI_API_KEY)" ] && [ -z "$$OPENAI_API_KEY" ]; then \
		echo "Error: OPENAI_API_KEY not set."; \
		echo "Usage: make test-integration OPENAI_API_KEY=your-key OPENAI_MODEL=your-model"; \
		exit 1; \
	fi
	@go test -v ./llm -run Integration -timeout 120s

## test-all: Run all tests including integration
test-all: test test-integration

## run: Build and run the binary from build directory
run: build
	@echo "Running $(BINARY_NAME) from $(BUILD_DIR)/..."
	@./$(BUILD_DIR)/$(BINARY_NAME)

## dev: Run in development mode (does not create binary in project root)
dev:
	@echo "Running in development mode..."
	@echo "(Running 'go run' - no binary will be left in project directory)"
	@go run ./cmd/kimi

## verify: Run full verification
verify:
	@echo "Running full verification..."
	@./scripts/verify.sh

## benchmark: Run agent benchmark (requires API keys)
benchmark:
	@if [ -z "$$OPENAI_API_KEY" ]; then \
		echo "Error: Set OPENAI_BASE_URL, OPENAI_API_KEY, OPENAI_MODEL first."; \
		echo "  source scripts/env.sh"; \
		exit 1; \
	fi
	@echo "Running agent benchmark..."
	@mkdir -p $(CURDIR)/benchmark_results
	@RUN_BENCHMARK=1 BENCHMARK_OUTPUT_DIR=$(CURDIR)/benchmark_results go test -v -count=1 -timeout 600s ./internal/benchmark/ -run TestBenchmark_DefaultCases
	@echo ""
	@echo "Results saved to benchmark_results/"

## docker-build: Build Docker image (if Dockerfile exists)
docker-build:
	@if [ -f Dockerfile ]; then \
		docker build -t $(BINARY_NAME):$(VERSION) .; \
	else \
		echo "Dockerfile not found, skipping Docker build..."; \
	fi

# Default target
.DEFAULT_GOAL := help
