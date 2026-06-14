.PHONY: help build run test clean docker-build docker-run docker-stop dev setup deps lint format check

# Default target
help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

# Build configuration
BINARY_NAME=annet-oil
BUILD_DIR=./bin
MAIN_PATH=./cmd/annet-oil
VERSION?=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date +%FT%T%z)
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Go configuration
GOCMD=go
GOBUILD=$(GOCMD) build
GOMOD=$(GOCMD) mod
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOCLEAN=$(GOCMD) clean
GOVET=$(GOCMD) vet
GOFMT=gofmt

# Setup development environment
setup: ## Setup development environment
	@echo "Setting up development environment..."
	$(GOMOD) download
	@mkdir -p $(BUILD_DIR)
	@mkdir -p keys
	@mkdir -p annet-configs/{default,telnet,orion}
	@mkdir -p annet-data/{default,telnet,orion}
	@echo "Development environment setup complete!"

# Install dependencies
deps: ## Download and install dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) verify
	$(GOMOD) tidy

# Build the application
build: deps ## Build the application
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Build for multiple platforms
build-all: deps ## Build for multiple platforms
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	@echo "Multi-platform build complete!"

# Run the application
run: build ## Build and run the application
	@echo "Starting $(BINARY_NAME)..."
	$(BUILD_DIR)/$(BINARY_NAME)

# Run in development mode
dev: ## Run in development mode with live reload
	@echo "Starting development server..."
	$(GOCMD) run $(MAIN_PATH) server start

# Run tests
test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

# Run tests with coverage report
test-coverage: test ## Run tests and generate coverage report
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Lint the code
lint: ## Run linter
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		$(GOVET) ./...; \
	fi

# Format the code
format: ## Format Go code
	@echo "Formatting code..."
	$(GOFMT) -s -w .

# Check code formatting
check: ## Check if code is properly formatted
	@echo "Checking code formatting..."
	@if [ -n "$$($(GOFMT) -l .)" ]; then \
		echo "Code is not properly formatted. Run 'make format' to fix."; \
		exit 1; \
	else \
		echo "Code is properly formatted."; \
	fi

# Clean build artifacts
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Docker commands
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t annet-oil:latest .

docker-run: docker-build ## Build and run Docker container
	@echo "Running Docker container..."
	docker-compose up -d

docker-stop: ## Stop Docker containers
	@echo "Stopping Docker containers..."
	docker-compose down

docker-logs: ## Show Docker container logs
	@echo "Showing Docker logs..."
	docker-compose logs -f annet-oil

# Development shortcuts
start-api: build ## Start only API server
	$(BUILD_DIR)/$(BINARY_NAME) server api

start-ssh: build ## Start only SSH server
	$(BUILD_DIR)/$(BINARY_NAME) server ssh

gen: build ## Run gen command (example: make gen ARGS="-g router1.example.com")
	$(BUILD_DIR)/$(BINARY_NAME) gen $(ARGS)

diff: build ## Run diff command (example: make diff ARGS="-g router1.example.com")
	$(BUILD_DIR)/$(BINARY_NAME) diff $(ARGS)

containers: build ## List containers status
	$(BUILD_DIR)/$(BINARY_NAME) containers list

routing: build ## Show routing table
	$(BUILD_DIR)/$(BINARY_NAME) routing show

# Installation
install: build ## Install binary to system path
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "Installation complete!"

uninstall: ## Uninstall binary from system
	@echo "Uninstalling $(BINARY_NAME)..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Uninstallation complete!"

# Release
release: clean test lint build-all ## Prepare release
	@echo "Release preparation complete!"
	@echo "Binaries available in $(BUILD_DIR)/"

# Show current version
version: ## Show current version
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"