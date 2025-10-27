.PHONY: help build run clean test deps docker-up docker-down migrate-up migrate-down

# Default target
help:
	@echo "Available commands:"
	@echo "  deps        - Install dependencies"
	@echo "  docker-up   - Start development infrastructure"
	@echo "  docker-down - Stop development infrastructure"
	@echo "  migrate-up  - Run database migrations"
	@echo "  migrate-down- Rollback database migrations"
	@echo "  build       - Build the gateway service"
	@echo "  run         - Run the gateway service"
	@echo "  test        - Run tests"
	@echo "  clean       - Clean build artifacts"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	cd services/gateway && go mod tidy
	cd services/gateway && go mod download

# Start development infrastructure (PostgreSQL, Redis, Meilisearch)
docker-up:
	@echo "Starting development infrastructure..."
	docker compose -f deployment/dependencies/docker-compose.yml up -d

# Stop development infrastructure
docker-down:
	@echo "Stopping development infrastructure..."
	docker compose -f deployment/dependencies/docker-compose.yml down

# Run database migrations up
migrate-up:
	@echo "Running database migrations..."
	@if ! command -v migrate &> /dev/null; then \
		echo "Installing migrate tool..."; \
		go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest; \
	fi
	migrate -path ../../DB/migrations -database "postgres://growthmind:growthmind123@localhost:5432/growthmind?sslmode=disable" up

# Run database migrations down
migrate-down:
	@echo "Rolling back database migrations..."
	@if ! command -v migrate &> /dev/null; then \
		echo "Installing migrate tool..."; \
		go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest; \
	fi
	migrate -path ../../DB/migrations -database "postgres://growthmind:growthmind123@localhost:5432/growthmind?sslmode=disable" down

# Build the gateway service
build:
	@echo "Building gateway service..."
	cd services/gateway && go build -o ../../bin/gateway api.go

# Run the gateway service
run: build
	@echo "Running gateway service..."
	./bin/gateway -f services/gateway/etc/api.yaml

# Run tests
test:
	@echo "Running tests..."
	cd services/gateway && go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	cd services/gateway && go clean

# Development setup (run all setup steps)
setup: deps docker-up migrate-up
	@echo "Development environment setup complete!"

# Full development workflow
dev: setup run

# Build Docker image for gateway
docker-build:
	@echo "Building Docker image for gateway..."
	docker build -f deployment/docker/gateway/Dockerfile -t growthmind/gateway:latest .

# Quick restart (useful during development)
restart: docker-down docker-up migrate-up run
