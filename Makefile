BIN_DIR := ./build/bin
APP := $(BIN_DIR)/app
MIGRATE := $(BIN_DIR)/migrate
MAIN_PKG := ./cmd/app
MIGRATE_PKG := ./cmd/migrate
ENV_FILE := .env

GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint

.PHONY: all build clean migrate migrate-validate run test lint help

all: clean test build

$(GOLANGCI_LINT):
	@echo "Installing golangci-lint..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

build:
	@echo "Creating build directory..."
	mkdir -p $(BIN_DIR)
	@echo "Building $(APP)..."
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(APP) $(MAIN_PKG)
	@echo "Building $(MIGRATE)..."
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(MIGRATE) $(MIGRATE_PKG)
	@echo "Build completed!"

clean:
	@echo "Cleaning build directory..."
	rm -rf $(BIN_DIR)
	@echo "Clean completed!"

migrate: build
	@echo "Applying PostgreSQL migrations with environment from $(ENV_FILE)..."
	@if [ -f "$(ENV_FILE)" ]; then \
		export $$(grep -v '^#' $(ENV_FILE) | xargs) && $(MIGRATE) up; \
	else \
		echo "Warning: $(ENV_FILE) not found, running without environment file"; \
		$(MIGRATE) up; \
	fi

migrate-validate: build
	@echo "Validating PostgreSQL migrations with environment from $(ENV_FILE)..."
	@if [ -f "$(ENV_FILE)" ]; then \
		export $$(grep -v '^#' $(ENV_FILE) | xargs) && $(MIGRATE) validate; \
	else \
		echo "Warning: $(ENV_FILE) not found, running without environment file"; \
		$(MIGRATE) validate; \
	fi

run: migrate
	@echo "Running $(APP) with environment from $(ENV_FILE)..."
	@if [ -f "$(ENV_FILE)" ]; then \
		export $$(grep -v '^#' $(ENV_FILE) | xargs) && $(APP); \
	else \
		echo "Warning: $(ENV_FILE) not found, running without environment file"; \
		$(APP); \
	fi

test:
	@echo "Running tests..."
	go test -race ./... -v
	@echo "Tests completed!"

lint: $(GOLANGCI_LINT)
	@echo "Running linter..."
	$(GOLANGCI_LINT) run
	@echo "Linting completed!"

docker-build:
	@echo "Building Docker image..."
	docker build -t goat-api .
	@echo "Docker build completed!"

docker-run: docker-build
	@echo "Running Docker container..."
	docker run -p 8080:8080 --env-file $(ENV_FILE) goat-api

help:
	@echo "$(GREEN)Available targets:"
	@echo "  all          - Clean, test, and build"
	@echo "  build        - Build the application"
	@echo "  clean        - Clean build artifacts"
	@echo "  migrate      - Build and apply PostgreSQL migrations"
	@echo "  migrate-validate - Verify PostgreSQL is at the embedded schema version"
	@echo "  run          - Migrate, then run the application"
	@echo "  test         - Run tests"
	@echo "  lint         - Run linter (optional)"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Build and run Docker container"
	@echo "  help         - Show this help message"

verify:
	@echo "Verifying Go version..."
	go version
	@echo "Verifying Go modules..."
	go mod verify
