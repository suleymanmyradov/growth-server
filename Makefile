.PHONY: deps docker-up docker-down migrate-up migrate-down generate-api format-api validate-api swagger-api generate-client-proto generate-auth-proto generate-search-proto generate-conversation-proto generate-client-repo generate-auth-repo generate-conversations-repo generate-search-repo sqlc build build-auth build-client build-search build-conversations build-gateway clean
SQLC_VERSION ?= v1.27.0
SQLC_SERVICES := auth client conversations search
# Default target
help:
	@echo "Available commands:"
	@echo "  deps                - Install dependencies"
	@echo "  docker-up           - Start development infrastructure"
	@echo "  docker-down         - Stop development infrastructure"
	@echo "  migrate-up          - Run database migrations"
	@echo "  migrate-down        - Rollback database migrations"
	@echo "  generate-api        - Generate API gateway from contract"
	@echo "  swagger-api         - Generate Swagger spec for the gateway"
	@echo "  generate-client-repo       - Generate client service repository layer"
	@echo "  generate-auth-repo        - Generate auth service repository layer"
	@echo "  generate-conversations-repo - Generate conversations service repository layer"
	@echo "  generate-search-repo       - Generate search service repository layer"
	@echo "  sqlc                 - Generate repository layer using sqlc"
	@echo "    SQLC_VERSION=$(SQLC_VERSION)"
	@echo "  build                - Build all services"
	@echo "  build-auth           - Build auth service"
	@echo "  build-client         - Build client service"
	@echo "  build-search         - Build search service"
	@echo "  build-conversations  - Build conversations service"
	@echo "  build-gateway        - Build gateway service"
	@echo "  clean                - Clean build artifacts"
	
deps:
	@echo "Installing dependencies..."
	go mod tidy
	go mod download

# Start development infrastructure (PostgreSQL, Redis, Meilisearch)
docker-up:
	@echo "Starting development infrastructure..."
	docker-compose -f deploy/docker-compose.yml up -d

# Stop development infrastructure
docker-down:
	@echo "Stopping development infrastructure..."
	docker-compose -f deploy/docker-compose.yml down

# Run database migrations up
migrate-up:
	@echo "Running database migrations..."
	@if ! command -v migrate &> /dev/null; then \
		echo "Installing migrate tool..."; \
		go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest; \
	fi
	migrate -path sql/migrations -database "postgres://growth:growth123@localhost:5432/growth?sslmode=disable" up

# Run database migrations down
migrate-down:
	@echo "Rolling back database migrations..."
	@if ! command -v migrate &> /dev/null; then \
		echo "Installing migrate tool..."; \
		go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest; \
	fi
	migrate -path migrations -database "postgres://growthmind:growthmind123@localhost:5434/growthmind?sslmode=disable" down

generate-api:
	@echo "Generating API gateway..."
	goctl api go -api ./services/gateway/contract/main.api -dir ./services/gateway/growth -style goZero
format-api:
	@echo "Formatting API gateway..."
	goctl api format --dir ./services/gateway/contract/main.api
validate-api:
	@echo "Validating API gateway..."
	goctl api validate -api ./services/gateway/contract/main.api
swagger-api:
	@echo "Generating Swagger spec for gateway..."
	@mkdir -p ./services/gateway/contract/swagger
	goctl api swagger -api ./services/gateway/contract/main.api -dir ./services/gateway/contract/swagger -filename gateway.swagger.json
sqlc:
	@echo "Generating repository layer with sqlc..."
	@if ! command -v sqlc >/dev/null 2>&1; then \
		echo "Installing sqlc $(SQLC_VERSION)..."; \
		go install github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION); \
	fi
	@for svc in $(SQLC_SERVICES); do \
		echo "---- $$svc"; \
		mkdir -p services/microservices/$$svc/rpc/internal/repository/db; \
		mkdir -p services/microservices/$$svc/rpc/internal/repository/cache; \
		if ls sql/queries/$$svc/*.sql >/dev/null 2>&1; then \
			sqlc generate -f sql/conf/sqlc.$$svc.yaml; \
		else \
			echo "skip: no queries in sql/queries/$$svc"; \
		fi; \
	done
generate-client-proto:
	@echo "Generating client proto..."
	goctl rpc protoc ./services/microservices/client/api/v1/client.proto --go_out=./services/microservices/client/rpc/pb --go-grpc_out=./services/microservices/client/rpc/pb --zrpc_out=./services/microservices/client/rpc -m --style goZero
generate-auth-proto:
	@echo "Generating auth proto..."
	goctl rpc protoc ./services/microservices/auth/api/v1/auth.proto --go_out=./services/microservices/auth/rpc/pb --go-grpc_out=./services/microservices/auth/rpc/pb --zrpc_out=./services/microservices/auth/rpc --style goZero
generate-search-proto:
	@echo "Generating search proto..."
	goctl rpc protoc ./services/microservices/search/api/v1/search.proto --go_out=./services/microservices/search/rpc/pb --go-grpc_out=./services/microservices/search/rpc/pb --zrpc_out=./services/microservices/search/rpc --style goZero
generate-conversations-proto:
	@echo "Generating conversations proto..."
	goctl rpc protoc ./services/microservices/conversations/api/v1/conversations.proto --go_out=./services/microservices/conversations/rpc/pb --go-grpc_out=./services/microservices/conversations/rpc/pb --zrpc_out=./services/microservices/conversations/rpc -m --style goZero

# Build commands
build: build-auth build-client build-search build-conversations build-gateway
	@echo "All services built successfully!"

build-auth:
	@echo "Building auth service..."
	@mkdir -p bin
	go build -o bin/auth ./services/microservices/auth/rpc

build-client:
	@echo "Building client service..."
	@mkdir -p bin
	go build -o bin/client ./services/microservices/client/rpc

build-search:
	@echo "Building search service..."
	@mkdir -p bin
	go build -o bin/search ./services/microservices/search/rpc

build-conversations:
	@echo "Building conversations service..."
	@mkdir -p bin
	go build -o bin/conversations ./services/microservices/conversations/rpc

build-gateway:
	@echo "Building gateway service..."
	@mkdir -p bin
	go build -o bin/gateway ./services/gateway/growth

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin
	@rm -rf logs