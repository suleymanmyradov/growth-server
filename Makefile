.PHONY: deps docker-up docker-down migrate-up migrate-down generate-api format-api validate-api swagger-api open-swagger generate-client-proto generate-auth-proto generate-search-proto generate-notification-proto generate-ai-coach-proto generate-filemanager-proto generate-client-repo generate-auth-repo generate-search-repo sqlc lint build build-auth build-client build-search build-notifications build-ai-coach build-filemanager build-search-sync build-gateway build-billing-reconciler clean run-auth run-client run-search run-aicoach run-filemanager run-gateway run-all tmux-start tmux-stop tmux-attach
SQLC_VERSION ?= v1.27.0
SQLC_SERVICES := auth client search notifications
# Default target
help:
	@echo "Available commands:"
	@echo "  deps                - Install dependencies"
	@echo "  docker-up           - Start development dependencies (Postgres, Redis, Redpanda, Meilisearch)"
	@echo "  docker-down         - Stop development dependencies"
	@echo "  run-auth            - Run auth service locally"
	@echo "  run-client          - Run client service locally"
	@echo "  run-search          - Run search service locally"
	@echo "  run-aicoach         - Run ai-coach RPC service locally"
	@echo "  run-notifications   - Run notifications service locally"
	@echo "  run-ai-coach-consumer - Run ai-coach consumer locally"
	@echo "  run-search-sync     - Run search-sync worker locally"
	@echo "  run-gateway         - Run API gateway locally"
	@echo "  run-all             - Run all services locally"
	@echo "  tmux-start          - Start all services in a tmux session (no binaries)"
	@echo "  tmux-stop           - Stop the tmux session"
	@echo "  tmux-attach         - Attach to the running tmux session"
	@echo "  migrate-up          - Run database migrations"
	@echo "  migrate-down        - Rollback database migrations"
	@echo "  generate-api        - Generate API gateway from contract"
	@echo "  swagger-api         - Generate Swagger spec for the gateway"
	@echo "  open-swagger        - Open Swagger UI in browser"
	@echo "  generate-client-repo       - Generate client service repository layer"
	@echo "  generate-auth-repo        - Generate auth service repository layer"
	@echo "  generate-search-repo       - Generate search service repository layer"
	@echo "  sqlc                 - Generate repository layer using sqlc"
	@echo "    SQLC_VERSION=$(SQLC_VERSION)"
	@echo "  build                - Build all services"
	@echo "  build-auth           - Build auth service"
	@echo "  build-client         - Build client service"
	@echo "  build-search         - Build search service"
	@echo "  build-notifications  - Build notifications service"
	@echo "  build-ai-coach       - Build ai-coach service"
	@echo "  build-search-sync    - Build search-sync service"
	@echo "  build-gateway        - Build gateway service"
	@echo "  build-billing-reconciler - Build billing-reconciler command"
	@echo "  lint                 - Run golangci-lint"
	@echo "  clean                - Clean build artifacts"
	
deps:
	@echo "Installing dependencies..."
	go mod tidy
	go mod download

# Start development dependencies (PostgreSQL, Redis, Redpanda, Meilisearch)
docker-up:
	@echo "Starting development dependencies..."
	docker compose -f deploy/docker-compose.yml up -d

# Stop development dependencies
docker-down:
	@echo "Stopping development dependencies..."
	docker compose -f deploy/docker-compose.yml down

# Run notifications service locally (connects to docker dependencies)
run-notifications: build-notifications
	@echo "Running notifications service..."
	@mkdir -p logs
	./bin/notifications -f services/microservices/notifications/rpc/etc/notifications.yaml

# Run ai-coach consumer locally (connects to docker dependencies)
run-ai-coach: build-ai-coach
	@echo "Running ai-coach consumer..."
	@mkdir -p logs
	./bin/ai-coach-consumer -f services/microservices/ai-coach-consumer/etc/ai-coach.yaml

# Run search-sync worker locally (connects to docker dependencies)
run-search-sync: build-search-sync
	@echo "Running search-sync worker..."
	@mkdir -p logs
	./bin/search-sync -f services/microservices/search-sync/etc/search-sync.yaml

# Run auth service locally (connects to docker dependencies)
run-auth: build-auth
	@echo "Running auth service..."
	@mkdir -p logs
	./bin/auth -f services/microservices/auth/rpc/etc/auth.yaml

# Run client service locally (connects to docker dependencies)
run-client: build-client
	@echo "Running client service..."
	@mkdir -p logs
	./bin/client -f services/microservices/client/rpc/etc/client.yaml

# Run search service locally (connects to docker dependencies)
run-search: build-search
	@echo "Running search service..."
	@mkdir -p logs
	./bin/search -f services/microservices/search/rpc/etc/search.yaml

# Run ai-coach RPC service locally (connects to docker dependencies)
run-aicoach: build-ai-coach
	@echo "Running ai-coach RPC service..."
	@mkdir -p logs
	./bin/ai-coach -f services/microservices/ai-coach/rpc/etc/aicoach.yaml

# Run ai-coach consumer locally (connects to docker dependencies)
run-ai-coach-consumer: build-ai-coach
	@echo "Running ai-coach consumer..."
	@mkdir -p logs
	./bin/ai-coach-consumer -f services/microservices/ai-coach-consumer/etc/ai-coach.yaml

# Run filemanager locally (connects to docker dependencies)
run-filemanager: build-filemanager
	@echo "Running filemanager service..."
	@mkdir -p logs
	./bin/filemanager -f services/microservices/filemanager/rpc/etc/filemanager.yaml

# Run gateway locally (connects to docker dependencies)
run-gateway: build-gateway
	@echo "Running API gateway..."
	@mkdir -p logs
	./bin/gateway -f services/gateway/growth/etc/growthapi.yaml

# Run all services locally
run-all: build
	@echo "Running all services..."
	@mkdir -p logs
	./bin/auth -f services/microservices/auth/rpc/etc/auth.yaml > logs/auth.log 2>&1 &
	./bin/client -f services/microservices/client/rpc/etc/client.yaml > logs/client.log 2>&1 &
	./bin/search -f services/microservices/search/rpc/etc/search.yaml > logs/search.log 2>&1 &
	./bin/ai-coach -f services/microservices/ai-coach/rpc/etc/aicoach.yaml > logs/ai-coach.log 2>&1 &
	./bin/filemanager -f services/microservices/filemanager/rpc/etc/filemanager.yaml > logs/filemanager.log 2>&1 &
	./bin/gateway -f services/gateway/growth/etc/growthapi.yaml > logs/gateway.log 2>&1 &
	./bin/ai-coach-consumer -f services/microservices/ai-coach-consumer/etc/ai-coach.yaml > logs/ai-coach-consumer.log 2>&1 &
	./bin/notifications -f services/microservices/notifications/rpc/etc/notifications.yaml > logs/notifications.log 2>&1 &
	./bin/search-sync -f services/microservices/search-sync/etc/search-sync.yaml > logs/search-sync.log 2>&1 &
	@echo "All services started. Logs are in the logs/ directory."
	@echo "To stop all services, run: pkill -f 'bin/(auth|client|search|ai-coach|ai-coach-consumer|filemanager|gateway|notifications|search-sync)'"

# Start all services in a tmux session and attach automatically
tmux-start:
	@./scripts/start-services.sh

# Stop the tmux session
tmux-stop:
	@tmux kill-session -t growth 2>/dev/null && echo "Session 'growth' stopped." || echo "No session 'growth' running."

# Attach to the running tmux session
tmux-attach:
	@tmux attach -t growth

# Run database migrations up
migrate-up:
	@echo "Running database migrations..."
	@if ! command -v migrate &> /dev/null; then \
		echo "Installing migrate tool..."; \
		go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest; \
	fi
	@if [ -z "$(DATABASE_URL)" ]; then \
		echo "Error: DATABASE_URL is not set"; \
		echo "Example: export DATABASE_URL=postgres://user:password@localhost:5434/dbname?sslmode=disable"; \
		exit 1; \
	fi
	migrate -path sql/migrations -database "$(DATABASE_URL)" up

# Run database migrations down
migrate-down:
	@echo "Rolling back database migrations..."
	@if ! command -v migrate &> /dev/null; then \
		echo "Installing migrate tool..."; \
		go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest; \
	fi
	@if [ -z "$(DATABASE_URL)" ]; then \
		echo "Error: DATABASE_URL is not set"; \
		echo "Example: export DATABASE_URL=postgres://user:password@localhost:5434/dbname?sslmode=disable"; \
		exit 1; \
	fi
	migrate -path sql/migrations -database "$(DATABASE_URL)" down

generate-api:
	@echo "Generating API gateway..."
	goctl api go -api ./services/gateway/contract/main.api -dir ./services/gateway/growth -style goZero
format-api:
	@echo "Formatting API gateway..."
	goctl api format --dir ./services/gateway/contract
validate-api:
	@echo "Validating API gateway..."
	goctl api validate -api ./services/gateway/contract/main.api
swagger-api:
	@echo "Generating Swagger spec for gateway..."
	@mkdir -p ./services/gateway/contract/swagger
	goctl api swagger -api ./services/gateway/contract/main.api -dir ./services/gateway/contract/swagger -filename swagger

open-swagger:
	@echo "Opening Swagger UI in browser..."
	bunx open-swagger-ui --open ./services/gateway/contract/swagger/swagger.json
	
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
generate-notification-proto:
	@echo "Generating notifications proto..."
	goctl rpc protoc ./services/microservices/notifications/api/v1/notifications.proto --go_out=./services/microservices/notifications/rpc/pb --go-grpc_out=./services/microservices/notifications/rpc/pb --zrpc_out=./services/microservices/notifications/rpc -m --style goZero
generate-ai-coach-proto:
	@echo "Generating ai-coach proto..."
	goctl rpc protoc ./services/microservices/ai-coach/api/v1/ai-coach.proto --go_out=./services/microservices/ai-coach/rpc/pb --go-grpc_out=./services/microservices/ai-coach/rpc/pb --zrpc_out=./services/microservices/ai-coach/rpc --style goZero
generate-filemanager-proto:
	@echo "Generating filemanager proto..."
	goctl rpc protoc ./services/microservices/filemanager/api/v1/filemanager.proto --go_out=./services/microservices/filemanager/rpc/pb --go-grpc_out=./services/microservices/filemanager/rpc/pb --zrpc_out=./services/microservices/filemanager/rpc --style goZero

# Lint with golangci-lint (Uber Go Style Guide recommended)
lint:
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest; \
	fi
	golangci-lint run ./...

# Build commands
build: build-auth build-client build-search build-notifications build-ai-coach build-filemanager build-search-sync build-gateway build-billing-reconciler
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

build-notifications:
	@echo "Building notifications service..."
	@mkdir -p bin
	go build -o bin/notifications ./services/microservices/notifications/rpc

build-ai-coach:
	@echo "Building ai-coach service..."
	@mkdir -p bin
	go build -o bin/ai-coach ./services/microservices/ai-coach/rpc
	go build -o bin/ai-coach-consumer ./services/microservices/ai-coach-consumer

build-filemanager:
	@echo "Building filemanager service..."
	@mkdir -p bin
	go build -o bin/filemanager ./services/microservices/filemanager/rpc

build-search-sync:
	@echo "Building search-sync service..."
	@mkdir -p bin
	go build -o bin/search-sync ./services/microservices/search-sync

build-gateway:
	@echo "Building gateway service..."
	@mkdir -p bin
	go build -o bin/gateway ./services/gateway/growth

build-billing-reconciler:
	@echo "Building billing-reconciler..."
	@mkdir -p bin
	go build -o bin/billing-reconciler ./services/microservices/billing-reconciler

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin
	@rm -rf logs