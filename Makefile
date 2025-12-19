BIN_DIR := ./build/bin
APP := $(BIN_DIR)/app
MAIN_PKG := ./cmd/app
ENV_FILE := .env

WIRE := $(shell go env GOPATH)/bin/wire
GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint

.PHONY: all build clean run test lint wire help

all: clean test build

$(WIRE):
	@echo "Installing Wire..."
	go install github.com/google/wire/cmd/wire@latest

$(GOLANGCI_LINT):
	@echo "Installing golangci-lint..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

wire: $(WIRE)
	@echo "Generating Wire bindings..."
	$(WIRE) $(MAIN_PKG)
	@echo "Wire generation completed!"

build: wire
	@echo "Creating build directory..."
	mkdir -p $(BIN_DIR)
	@echo "Building $(APP)..."
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(APP) $(MAIN_PKG)
	@echo "Build completed!"

clean:
	@echo "Cleaning build directory..."
	rm -rf $(BIN_DIR)
	@echo "Clean completed!"

run: build
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
	@echo "  build        - Build the application (includes Wire generation)"
	@echo "  clean        - Clean build artifacts"
	@echo "  run          - Build and run the application"
	@echo "  test         - Run tests"
	@echo "  wire         - Generate Wire bindings only"
	@echo "  lint         - Run linter (optional)"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Build and run Docker container"
	@echo "  help         - Show this help message"

verify:
	@echo "Verifying Go version..."
	go version
	@echo "Verifying Go modules..."
	go mod verify
	@echo "Verifying Wire installation..."
	@if [ -f "$(WIRE)" ]; then \
		echo "Wire installed at: $(WIRE)"; \
	else \
		echo "Wire not installed - will be installed during build"; \
	fi