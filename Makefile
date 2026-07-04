.PHONY: help build build-api build-mcp run run-api run-mcp run-all stop stop-api stop-mcp stop-all restart restart-api restart-mcp test clean docker-build docker-run docker-stop dev setup deps lint format check watch

# Default target
help: ## Show this help message
	@echo "╔══════════════════════════════════════════════════════════════╗"
	@echo "║                    Annet Oil - Makefile                       ║"
	@echo "╚══════════════════════════════════════════════════════════════╝"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Quick Start:"
	@echo "  make setup        - Setup development environment"
	@echo "  make build        - Build all components"
	@echo "  make run          - Run API + MCP"
	@echo "  make run-all      - Run complete stack"
	@echo "  make stop         - Stop all services"

# Build configuration
BINARY_NAME=annet-oil-server
BUILD_DIR=.
MAIN_PATH=./cmd/annet-oil
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date +%FT%T%z)
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Docker configuration
MCP_COMPOSE=docker-compose.mcp.yml
ANNET_COMPOSE=docker-compose.yml

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
setup: env deps ## Setup development environment
	@echo "Setting up development environment..."
	@mkdir -p keys
	@mkdir -p annet-configs/{default,telnet,orion}
	@mkdir -p annet-data/{default,telnet,orion}
	@echo "Development environment setup complete!"

env: ## Create .env from template
	@if [ ! -f .env ]; then \
		echo "Creating .env from template..."; \
		cp .env.example .env; \
		echo "Please edit .env to set ANNET_OIL_AUTH_TOKEN"; \
	else \
		echo ".env already exists"; \
	fi

check-env: ## Check if .env exists
	@if [ ! -f .env ]; then \
		echo "Error: .env file not found!"; \
		echo "Run 'make env' first"; \
		exit 1; \
	fi

# Install dependencies
deps: ## Download and install dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) verify
	$(GOMOD) tidy

# Build targets
build: build-api build-mcp ## Build everything (API + MCP)
	@echo "Build complete!"

build-api: deps ## Build Annet Oil API
	@echo "Building $(BINARY_NAME)..."
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_PATH)
	@echo "API binary built: $(BINARY_NAME)"

build-mcp: ## Build MCP Docker container
	@echo "Building MCP container..."
	docker-compose -f $(MCP_COMPOSE) build
	@echo "MCP container built!"

# Build for multiple platforms
build-all: deps ## Build for multiple platforms
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	@echo "Multi-platform build complete!"

# Run targets
run: run-api run-mcp ## Run API on host + MCP in container
	@echo "All services started!"

run-bg: run-api-bg run-mcp ## Run API (background) + MCP in container
	@echo "All services started in background!"
	@sleep 2
	@$(MAKE) health

run-api: build-api check-env ## Run Annet Oil API on host
	@echo "Starting Annet Oil API..."
	./$(BINARY_NAME) server start

run-api-bg: build-api check-env ## Run Annet Oil API in background
	@echo "Starting Annet Oil API in background..."
	@nohup ./$(BINARY_NAME) server start > annet-oil.log 2>&1 & echo $$! > annet-oil.pid
	@echo "API started in background (PID: $$(cat annet-oil.pid))"
	@echo "Check annet-oil.log for output"
	@sleep 2
	@curl -s http://localhost:8080/health > /dev/null 2>&1 && echo "✓ API is running" || echo "⚠ API may still be starting..."

run-mcp: check-env ## Run MCP container
	@echo "Starting MCP container..."
	docker-compose -f $(MCP_COMPOSE) up -d
	@echo "MCP container started!"

run-all: run-annet run-api run-mcp ## Run complete stack
	@echo "Complete stack started!"

run-annet: ## Run Annet containers
	@echo "Starting Annet containers..."
	docker-compose -f $(ANNET_COMPOSE) up -d annet-default annet-telnet annet-orion
	@echo "Annet containers started!"

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

# Stop targets
stop: stop-mcp stop-api ## Stop all services
	@echo "All services stopped!"

stop-all: stop-annet stop-mcp stop-api ## Stop complete stack
	@echo "Complete stack stopped!"

stop-api: ## Stop Annet Oil API
	@echo "Stopping API..."
	@if [ -f annet-oil.pid ]; then \
		kill $$(cat annet-oil.pid) 2>/dev/null && echo "API stopped (PID: $$(cat annet-oil.pid))" || echo "Process not found"; \
		rm -f annet-oil.pid; \
	else \
		pkill -f $(BINARY_NAME) 2>/dev/null && echo "API stopped" || echo "API not running"; \
	fi

stop-mcp: ## Stop MCP container
	@echo "Stopping MCP container..."
	docker-compose -f $(MCP_COMPOSE) down

stop-annet: ## Stop Annet containers
	@echo "Stopping Annet containers..."
	docker-compose -f $(ANNET_COMPOSE) stop annet-default annet-telnet annet-orion

# Restart targets
restart: stop run ## Restart API + MCP
	@echo "Services restarted!"

restart-all: stop-all run-all ## Restart complete stack
	@echo "Complete stack restarted!"

restart-api: stop-api run-api ## Restart API only
	@echo "API restarted!"

restart-mcp: stop-mcp run-mcp ## Restart MCP only
	@echo "MCP restarted!"

restart-annet: stop-annet run-annet ## Restart Annet containers
	@echo "Annet containers restarted!"

# Clean build artifacts
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html
	docker-compose -f $(MCP_COMPOSE) down --rmi local 2>/dev/null || true

clean-all: clean ## Deep clean including volumes
	docker-compose -f $(MCP_COMPOSE) down -v
	docker-compose -f $(ANNET_COMPOSE) down -v
	rm -rf mcp-annet-oil/dist mcp-annet-oil/node_modules

# Docker commands
docker-build: build-mcp ## Build Docker images
	@echo "Building Docker images..."
	docker-compose -f $(ANNET_COMPOSE) build

docker-ps: ## Show running containers
	@docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -E "(mcp-annet-oil|annet-)" || echo "No containers running"

logs: ## Show all logs
	docker-compose -f $(MCP_COMPOSE) logs -f &
	docker-compose -f $(ANNET_COMPOSE) logs -f

logs-mcp: ## Show MCP logs
	docker-compose -f $(MCP_COMPOSE) logs -f

logs-annet: ## Show Annet logs
	docker-compose -f $(ANNET_COMPOSE) logs -f annet-default annet-telnet annet-orion

logs-api: ## Show API logs (if running in background)
	@if [ -f annet-oil.log ]; then \
		tail -f annet-oil.log; \
	else \
		echo "No log file found. API might be running in foreground."; \
	fi

# Development shortcuts
exec-mcp: ## Connect to MCP via stdio
	docker exec -it mcp-annet-oil node dist/index.js

shell-mcp: ## Open shell in MCP container
	docker exec -it mcp-annet-oil sh

shell-annet: ## Open shell in Annet container (use CONTAINER=annet-telnet for specific)
	docker exec -it $${CONTAINER:-annet-default} sh

health: ## Check service health
	@echo "═══════════════════════════════════"
	@echo "     Service Health Check"
	@echo "═══════════════════════════════════"
	@echo -n "API Server:       "
	@curl -s http://localhost:8080/health > /dev/null 2>&1 && echo "✓ Running" || echo "✗ Not running"
	@echo -n "MCP Container:    "
	@docker ps | grep -q mcp-annet-oil && echo "✓ Running" || echo "✗ Not running"
	@echo -n "Annet Default:    "
	@docker ps | grep -q annet-default && echo "✓ Running" || echo "✗ Not running"
	@echo -n "Annet Telnet:     "
	@docker ps | grep -q annet-telnet && echo "✓ Running" || echo "✗ Not running"
	@echo -n "Annet Orion:      "
	@docker ps | grep -q annet-orion && echo "✓ Running" || echo "✗ Not running"
	@echo "═══════════════════════════════════"

status: health docker-ps ## Show full system status

watch: ## Watch for file changes and rebuild (requires fswatch)
	@echo "Watching for changes..."
	@if command -v fswatch >/dev/null 2>&1; then \
		fswatch -o . -e ".*" -i "\\.go$$" | xargs -n1 -I{} make build-api; \
	else \
		echo "fswatch not found. Install with: brew install fswatch"; \
		exit 1; \
	fi

ports: ## Show exposed ports
	@echo "═══════════════════════════════════"
	@echo "     Exposed Ports"
	@echo "═══════════════════════════════════"
	@echo "API:    http://localhost:8080"
	@echo "SSH:    ssh://localhost:2222"
	@echo "═══════════════════════════════════"

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

# Systemd service management
service-install: ## Install systemd service
	@echo "Installing systemd service..."
	@sudo cp annet-oil.service /etc/systemd/system/
	@sudo systemctl daemon-reload
	@echo "Service installed. Use 'make service-enable' to enable auto-start"

service-enable: ## Enable systemd service
	@echo "Enabling systemd service..."
	@sudo systemctl enable annet-oil.service
	@echo "Service enabled for auto-start"

service-start: ## Start systemd service
	@echo "Starting systemd service..."
	@sudo systemctl start annet-oil.service
	@sleep 2
	@sudo systemctl status annet-oil.service --no-pager

service-stop: ## Stop systemd service
	@echo "Stopping systemd service..."
	@sudo systemctl stop annet-oil.service

service-restart: ## Restart systemd service
	@echo "Restarting systemd service..."
	@sudo systemctl restart annet-oil.service
	@sleep 2
	@sudo systemctl status annet-oil.service --no-pager

service-status: ## Check systemd service status
	@sudo systemctl status annet-oil.service --no-pager

service-logs: ## Show systemd service logs
	@sudo journalctl -u annet-oil.service -f

service-disable: ## Disable systemd service
	@echo "Disabling systemd service..."
	@sudo systemctl disable annet-oil.service
	@echo "Service disabled"

service-uninstall: service-stop service-disable ## Uninstall systemd service
	@echo "Uninstalling systemd service..."
	@sudo rm -f /etc/systemd/system/annet-oil.service
	@sudo systemctl daemon-reload
	@echo "Service uninstalled"

# Release
release: clean test lint build-all ## Prepare release
	@echo "Release preparation complete!"
	@echo "Binaries available in $(BUILD_DIR)/"

# Show current version
version: ## Show current version
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"

# Docker utility commands
docker-up: ## Start all Docker services
	docker-compose -f $(ANNET_COMPOSE) up -d
	docker-compose -f $(MCP_COMPOSE) up -d

docker-down: ## Stop all Docker services
	docker-compose -f $(MCP_COMPOSE) down
	docker-compose -f $(ANNET_COMPOSE) down

docker-restart: docker-down docker-up ## Restart all Docker services
	@echo "Docker services restarted!"

docker-pull: ## Pull latest Docker images
	docker-compose -f $(ANNET_COMPOSE) pull
	docker-compose -f $(MCP_COMPOSE) pull

docker-prune: ## Remove unused Docker resources
	docker system prune -af --volumes

# Debugging commands
debug: build-api ## Run API with debug output
	ANNET_OIL_DEBUG=true ./$(BINARY_NAME) server start

debug-mcp: ## Show MCP container details
	@docker inspect mcp-annet-oil | jq '.[0] | {State, Config: {Env: .Config.Env, Cmd: .Config.Cmd}}'

validate: ## Validate configuration files
	@echo "Validating docker-compose files..."
	@docker-compose -f $(ANNET_COMPOSE) config > /dev/null && echo "✓ $(ANNET_COMPOSE) is valid" || echo "✗ $(ANNET_COMPOSE) has errors"
	@docker-compose -f $(MCP_COMPOSE) config > /dev/null && echo "✓ $(MCP_COMPOSE) is valid" || echo "✗ $(MCP_COMPOSE) has errors"

# Backup and restore
backup: ## Backup data directories
	@echo "Creating backup..."
	@mkdir -p backups
	@tar czf backups/annet-oil-backup-$$(date +%Y%m%d-%H%M%S).tar.gz annet-configs annet-data keys
	@echo "Backup created in backups/"

restore: ## Restore from latest backup
	@if [ -z "$(BACKUP)" ]; then \
		LATEST=$$(ls -t backups/*.tar.gz 2>/dev/null | head -1); \
		if [ -z "$$LATEST" ]; then \
			echo "No backups found!"; \
			exit 1; \
		fi; \
		echo "Restoring from $$LATEST..."; \
		tar xzf $$LATEST; \
	else \
		echo "Restoring from $(BACKUP)..."; \
		tar xzf $(BACKUP); \
	fi
	@echo "Restore complete!"

# Quick commands for common tasks
quick-start: setup build run ## Quick start for new users
	@echo "Annet Oil is now running!"
	@echo "API: http://localhost:8080"
	@echo "Use 'make stop' to stop services"

update: ## Update dependencies and rebuild
	@echo "Updating dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "Rebuilding..."
	$(MAKE) build
	@echo "Update complete!"