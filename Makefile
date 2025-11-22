.PHONY: help build run clean test deps docker-up docker-down migrate-up migrate-down generate-api generate-proto

# Default target
help:
	@echo "Available commands:"
	@echo "  deps           - Install dependencies"
	@echo "  docker-up      - Start development infrastructure"
	@echo "  docker-down    - Stop development infrastructure"
	@echo "  migrate-up     - Run database migrations"
	@echo "  migrate-down   - Rollback database migrations"
	@echo "  generate-api   - Generate API gateway from contract"
	@echo "  generate-proto - Generate RPC services from proto files"
	@echo "  build          - Build the gateway service"
	@echo "  run            - Run the gateway service"
	@echo "  test           - Run tests"
	@echo "  clean          - Clean build artifacts"
	@echo "  setup          - Complete development setup"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod tidy
	go mod download

# Start development infrastructure (PostgreSQL, Redis, Meilisearch)
docker-up:
	@echo "Starting development infrastructure..."
	docker-compose -f deploy/dependencies/docker-compose.yml up -d

# Stop development infrastructure
docker-down:
	@echo "Stopping development infrastructure..."
	docker-compose -f deploy/dependencies/docker-compose.yml down

# Run database migrations up
migrate-up:
	@echo "Running database migrations..."
	@if ! command -v migrate &> /dev/null; then \
		echo "Installing migrate tool..."; \
		go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest; \
	fi
	migrate -path db/migrations -database "postgres://growthmind:growthmind123@localhost:5432/growthmind?sslmode=disable" up

# Run database migrations down
migrate-down:
	@echo "Rolling back database migrations..."
	@if ! command -v migrate &> /dev/null; then \
		echo "Installing migrate tool..."; \
		go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest; \
	fi
	migrate -path db/migrations -database "postgres://growthmind:growthmind123@localhost:5432/growthmind?sslmode=disable" down

# Generate API gateway from contract
generate-api:
	@echo "Generating API gateway..."
	goctl api go -api ./api/growth.api -dir ./services/gateway/api -style goZero

# Generate RPC services from proto files
generate-proto:
	@echo "Generating RPC services..."
	@echo "Generating auth service..."
	goctl rpc protoc ./services/gateway/services/auth/api/v1/auth.proto --go_out=./services/gateway/services/auth/rpc --go-grpc_out=./services/gateway/services/auth/rpc --zrpc_out=./services/gateway/services/auth/rpc --style goZero
	@echo "Generating habits service..."
	goctl rpc protoc ./services/gateway/services/habits/api/v1/habits.proto --go_out=./services/gateway/services/habits/rpc --go-grpc_out=./services/gateway/services/habits/rpc --zrpc_out=./services/gateway/services/habits/rpc --style goZero
	@echo "Generating goals service..."
	goctl rpc protoc ./services/gateway/services/goals/api/v1/goals.proto --go_out=./services/gateway/services/goals/rpc --go-grpc_out=./services/gateway/services/goals/rpc --zrpc_out=./services/gateway/services/goals/rpc --style goZero
	@echo "Generating articles service..."
	goctl rpc protoc ./services/gateway/services/articles/api/v1/articles.proto --go_out=./services/gateway/services/articles/rpc --go-grpc_out=./services/gateway/services/articles/rpc --zrpc_out=./services/gateway/services/articles/rpc --style goZero
	@echo "Generating conversations service..."
	goctl rpc protoc ./services/gateway/services/conversations/api/v1/conversations.proto --go_out=./services/gateway/services/conversations/rpc --go-grpc_out=./services/gateway/services/conversations/rpc --zrpc_out=./services/gateway/services/conversations/rpc --style goZero
	@echo "Generating search service..."
	goctl rpc protoc ./services/gateway/services/search/api/v1/search.proto --go_out=./services/gateway/services/search/rpc --go-grpc_out=./services/gateway/services/search/rpc --zrpc_out=./services/gateway/services/search/rpc --style goZero
	@echo "Generating saved service..."
	goctl rpc protoc ./services/gateway/services/saved/api/v1/saved.proto --go_out=./services/gateway/services/saved/rpc --go-grpc_out=./services/gateway/services/saved/rpc --zrpc_out=./services/gateway/services/saved/rpc --style goZero
	@echo "Generating activity service..."
	goctl rpc protoc ./services/gateway/services/activity/api/v1/activity.proto --go_out=./services/gateway/services/activity/rpc --go-grpc_out=./services/gateway/services/activity/rpc --zrpc_out=./services/gateway/services/activity/rpc --style goZero
	@echo "Generating notifications service..."
	goctl rpc protoc ./services/gateway/services/notifications/api/v1/notifications.proto --go_out=./services/gateway/services/notifications/rpc --go-grpc_out=./services/gateway/services/notifications/rpc --zrpc_out=./services/gateway/services/notifications/rpc --style goZero
	@echo "Generating settings service..."
	goctl rpc protoc ./services/gateway/services/settings/api/v1/settings.proto --go_out=./services/gateway/services/settings/rpc --go-grpc_out=./services/gateway/services/settings/rpc --zrpc_out=./services/gateway/services/settings/rpc --style goZero
	@echo "Generating report service..."
	goctl rpc protoc ./services/gateway/services/report/api/v1/report.proto --go_out=./services/gateway/services/report/rpc --go-grpc_out=./services/gateway/services/report/rpc --zrpc_out=./services/gateway/services/report/rpc --style goZero

# Build the gateway service
build:
	@echo "Building gateway service..."
	cd services/gateway && go build -o ../../bin/gateway api/growthapi.go

# Run the gateway service
run: build
	@echo "Running gateway service..."
	./bin/gateway -f services/gateway/api/etc/growthapi.yaml

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
setup: deps docker-up migrate-up generate-api
	@echo "Development environment setup complete!"

# Full development workflow
dev: setup run

# Build Docker image for gateway
docker-build:
	@echo "Building Docker image for gateway..."
	docker build -f deploy/docker/gateway/Dockerfile -t growthmind/gateway:latest .

# Quick restart (useful during development)
restart: docker-down docker-up migrate-up run
