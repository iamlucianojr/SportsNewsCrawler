.PHONY: help build test lint fmt vet clean run docker-build docker-up docker-down docker-logs coverage

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Build targets
build: ## Build the application
	@echo "Building application..."
	go build -o bin/server ./cmd/server
	go build -o bin/mock-feed ./cmd/mock-feed

build-all: ## Build all binaries
	@echo "Building all binaries..."
	go build -o bin/ ./...

install: ## Install dependencies
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

init-env: ## Create .env from .env.example
	@if [ -f .env ]; then \
		echo ".env already exists, skipping..."; \
	else \
		echo "Creating .env from .env.example..."; \
		cp .env.example .env; \
	fi

# Test targets
test: ## Run tests
	@echo "Running tests..."
	go test ./... -v

test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	go test -race ./... -v

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

coverage: test-coverage ## Alias for test-coverage

# Code quality targets
lint: ## Run golangci-lint
	@echo "Running linter..."
	golangci-lint run --timeout=5m ./...

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...

vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

check: fmt vet lint test ## Run all checks (fmt, vet, lint, test)

# Clean targets
clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean

# Run targets
run: ## Run the application locally
	@echo "Running application..."
	go run ./cmd/server

run-mock: ## Run the mock feed server
	@echo "Running mock feed server..."
	go run ./cmd/mock-feed

# Docker targets
docker-build: ## Build Docker images
	@echo "Building Docker images..."
	docker-compose build

docker-up: ## Start all services with docker-compose
	@echo "Starting services..."
	docker-compose up -d

docker-down: ## Stop all services
	@echo "Stopping services..."
	docker-compose down

docker-logs: ## Show docker-compose logs
	docker-compose logs -f

docker-restart: docker-down docker-up ## Restart all services

docker-clean: ## Remove all containers, volumes, and images
	@echo "Cleaning Docker resources..."
	docker-compose down -v
	docker system prune -f

# Development targets
dev: docker-up ## Start development environment
	@echo "Development environment started!"
	@echo "Services:"
	@echo "  - App:        http://localhost:8080"
	@echo "  - Mock Feed:  http://localhost:8081"
	@echo "  - Prometheus: http://localhost:9090"
	@echo "  - Jaeger:     http://localhost:16686"
	@echo "  - Kibana:     http://localhost:5601"
	@echo "  - Grafana:    http://localhost:3000"

dev-down: docker-down ## Stop development environment

# Database targets
mongo-shell: ## Connect to MongoDB shell
	docker exec -it sportsnewscrawler-mongodb-1 mongosh

purge-data: ## Purge all data (Mongo, Kafka, Prometheus, etc.) by removing volumes
	@echo "Purging all data..."
	docker-compose down -v
	@echo "Data purged. Run 'make up' to start fresh."


# Kafka targets
kafka-topics: ## List Kafka topics
	docker exec sportsnewscrawler-kafka kafka-topics --bootstrap-server localhost:9092 --list

kafka-consume: ## Consume from news_articles topic
	docker exec sportsnewscrawler-kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic news_articles --from-beginning

kafka-consume-dlq: ## Consume from DLQ topic
	docker exec sportsnewscrawler-kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic news_articles_dlq --from-beginning

# Monitoring targets
metrics: ## View Prometheus metrics
	@echo "Opening Prometheus..."
	open http://localhost:9090

traces: ## View Jaeger traces
	@echo "Opening Jaeger UI..."
	open http://localhost:16686

logs: ## View application logs
	docker-compose logs -f app

health: ## Check application health
	@curl -s http://localhost:8080/health || echo "Service not running"

# CI/CD targets
ci: check test-coverage ## Run CI pipeline locally

# Generate targets
generate: ## Generate mocks and code
	@echo "Generating code..."
	go generate ./...

# Dependency targets
deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

deps-vendor: ## Vendor dependencies
	@echo "Vendoring dependencies..."
	go mod vendor

# Documentation targets
docs: ## Open project documentation
	@echo "Opening README..."
	@cat README.md | less

# Quick commands
.PHONY: up down restart logs shell
up: docker-up ## Alias for docker-up
down: docker-down ## Alias for docker-down
restart: docker-restart ## Alias for docker-restart
logs: docker-logs ## Alias for docker-logs
