.PHONY: test test-docker test-coverage lint fmt help clean

# Default PostgreSQL URL for testing
TEST_DB_URL ?= postgres://postgres:postgres@localhost:6878/postgres?sslmode=disable

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

test: ## Run tests (requires PostgreSQL)
	@echo "Running tests..."
	@DATABASE_URL=$(TEST_DB_URL) go test -v ./...

test-docker: ## Run tests with Docker PostgreSQL
	@echo "Starting PostgreSQL in Docker..."
	@docker run -d --name migrator-test \
		-e POSTGRES_PASSWORD=postgres \
		-e POSTGRES_USER=postgres \
		-e POSTGRES_DB=postgres \
		-p 6878:5432 \
		postgres:17 > /dev/null 2>&1 || true
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 3
	@echo "Running tests..."
	@DATABASE_URL=$(TEST_DB_URL) go test -v ./...
	@echo "Cleaning up..."
	@docker stop migrator-test > /dev/null 2>&1 || true
	@docker rm migrator-test > /dev/null 2>&1 || true

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	@DATABASE_URL=$(TEST_DB_URL) go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...
	@echo "Generating HTML coverage report..."
	@go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"

lint: ## Run linters
	@echo "Running go vet..."
	@go vet ./...
	@echo "Running go fmt check..."
	@test -z "$$(gofmt -l .)" || (echo "Code is not formatted. Run 'make fmt'" && exit 1)

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...

build: ## Build example
	@echo "Building example..."
	@cd examples/basic && go build -o ../../bin/migrator-example

clean: ## Clean up test artifacts
	@echo "Cleaning up..."
	@rm -f coverage.txt coverage.html
	@rm -rf bin/
	@docker stop migrator-test > /dev/null 2>&1 || true
	@docker rm migrator-test > /dev/null 2>&1 || true

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy


