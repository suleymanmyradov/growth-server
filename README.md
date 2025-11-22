# GrowthMind Backend Server

A Go-based backend service for personal growth, habits, goals, and AI coaching built with go-zero framework.

## Tech Stack

- **Language**: Go 1.21+
- **Framework**: go-zero (REST gateway)
- **Database**: PostgreSQL
- **Cache**: Redis
- **Search**: Meilisearch
- **Authentication**: JWT tokens

## Project Structure

```
├── api/                    # API contract definitions
│   ├── self-dev.api       # Main API contract
│   └── types.api          # Type definitions
├── backend/
│   ├── services/
│   │   └── gateway/       # go-zero REST gateway
│   │       ├── api/       # API contracts
│   │       ├── etc/       # Configuration
│   │       ├── internal/  # Generated + custom code
│   │       │   ├── config/
│   │       │   ├── handler/
│   │       │   ├── logic/
│   │       │   ├── middleware/
│   │       │   ├── model/
│   │       │   ├── repository/
│   │       │   ├── svc/
│   │       │   └── types/
│   │       └── api.go     # Main entry point
│   ├── deployment/
│   │   └── dependencies/
│   │       └── docker-compose.yml
│   ├── bin/               # Build output
│   └── Makefile           # Development commands
├── DB/
│   └── migrations/        # Database migrations
└── go.mod                 # Go module definition
```

## Quick Start

### Prerequisites

- Go 1.21+
- Docker & Docker Compose
- PostgreSQL client (optional)
- protoc (for RPC generation)
- goctl (go-zero code generation tool)

### Setup

1. **Install dependencies:**
   ```bash
   # Install goctl
   go install github.com/zeromicro/go-zero/tools/goctl@latest
   
   # Install protoc and plugins
   brew install protobuf
   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
   ```

2. **Initialize the project:**
   ```bash
   make setup
   ```

3. **Start infrastructure:**
   ```bash
   make docker-up
   ```

4. **Run database migrations:**
   ```bash
   make migrate-up
   ```

5. **Build and run the service:**
   ```bash
   make run
   ```

The API server will start on `http://localhost:8888`

## Available Commands

```bash
make help          # Show all available commands
make deps          # Install dependencies
make generate-api  # Generate API gateway from contract
make generate-proto # Generate RPC services from proto files
make docker-up     # Start PostgreSQL, Redis, Meilisearch
make docker-down   # Stop infrastructure
make migrate-up    # Run database migrations
make migrate-down  # Rollback migrations
make build         # Build the gateway service
make run           # Run the gateway service
make test          # Run tests
make clean         # Clean build artifacts
make dev           # Full development workflow
```

## Project Structure

```
├── api/                           # API contract definitions
│   ├── growth.api                # Main API contract
│   └── types.api                 # Type definitions
├── backend/
│   ├── services/
│   │   └── gateway/
│   │       ├── api/              # Generated API gateway
│   │       │   ├── contract/     # API contracts
│   │       │   ├── etc/          # Configuration
│   │       │   ├── internal/     # Generated + custom code
│   │       │   └── growthapi.go  # Main entry point
│   │       └── services/         # RPC microservices
│   │           ├── auth/         # Authentication service
│   │           ├── habits/       # Habit tracking service
│   │           ├── goals/        # Goal management service
│   │           ├── articles/     # Article service
│   │           ├── conversations/ # AI conversation service
│   │           ├── notifications/ # Notification service
│   │           ├── search/       # Search service
│   │           └── profile/      # Profile service
│   ├── shared/                   # Shared components
│   │   ├── config/              # Configuration management
│   │   ├── middleware/          # Authentication middleware
│   │   ├── models/              # Database models
│   │   └── repository/          # Base repository
│   └── third_party/             # External integrations
│       ├── database/            # PostgreSQL connection
│       ├── cache/               # Redis connection
│       └── search/              # Meilisearch connection
├── DB/migrations/               # Database migrations
├── bin/                         # Build output
├── go.mod                       # Go module definition
└── Makefile                     # Development commands
```

## Development Workflow

### 1. API Development

1. **Update API contract** in `backend/services/gateway/api/contract/growth.api`
2. **Generate API gateway**: `make generate-api`
3. **Implement business logic** in generated handlers
4. **Test the API**: `make run`

### 2. RPC Service Development

1. **Create proto file** in `backend/services/gateway/services/{service}/api/v1/`
2. **Generate RPC service**: `make generate-proto`
3. **Implement service logic** in generated handlers
4. **Test the service**

### 3. Database Changes

1. **Create migration**: `migrate create -ext sql -dir DB/migrations -tz Local {name}`
2. **Write SQL migration** in the generated file
3. **Run migrations**: `make migrate-up`

## API Endpoints

### Authentication
- `POST /api/v1/auth/register` - User registration
- `POST /api/v1/auth/login` - User login
- `POST /api/v1/auth/refresh` - Refresh JWT token
- `POST /api/v1/auth/logout` - User logout

### Profile
- `GET /api/v1/profile/me` - Get current user profile
- `PUT /api/v1/profile` - Update user profile

### Habits
- `GET /api/v1/habits` - List user habits
- `POST /api/v1/habits` - Create new habit
- `GET /api/v1/habits/:id` - Get habit details
- `PUT /api/v1/habits/:id` - Update habit
- `DELETE /api/v1/habits/:id` - Delete habit
- `POST /api/v1/habits/:id/toggle` - Toggle habit completion
- `POST /api/v1/habits/reset-today` - Reset daily habits

### Goals
- `GET /api/v1/goals` - List user goals
- `POST /api/v1/goals` - Create new goal
- `GET /api/v1/goals/:id` - Get goal details
- `PUT /api/v1/goals/:id` - Update goal
- `DELETE /api/v1/goals/:id` - Delete goal
- `POST /api/v1/goals/:id/toggle` - Toggle goal completion
- `PUT /api/v1/goals/:id/progress` - Update goal progress

### Articles (Public)
- `GET /api/v1/articles` - List articles
- `GET /api/v1/articles/:id` - Get article details

### Saved Items
- `GET /api/v1/saved` - List saved items
- `POST /api/v1/saved` - Save item
- `DELETE /api/v1/saved/:id` - Remove saved item

### Conversations (AI Coach/Therapist)
- `GET /api/v1/conversations` - List conversations
- `POST /api/v1/conversations` - Start new conversation
- `GET /api/v1/conversations/:id` - Get conversation details
- `POST /api/v1/conversations/:id/messages` - Send message
- `GET /api/v1/conversations/:id/messages` - Get messages

### Search
- `GET /api/v1/search` - Search across content

### Notifications
- `GET /api/v1/notifications` - List notifications
- `PUT /api/v1/notifications/:id/read` - Mark notification as read
- `PUT /api/v1/notifications/read-all` - Mark all as read

### Activity
- `GET /api/v1/activity` - Get activity feed

### Settings
- `GET /api/v1/settings` - Get user settings
- `PUT /api/v1/settings` - Update user settings

### Report
- `POST /api/v1/report` - Submit feedback/abuse report

## Database Schema

The database includes the following main tables:
- `users` - User accounts
- `profiles` - User profiles with extended information
- `habits` - User habits with tracking
- `goals` - User goals with progress tracking
- `articles` - Content articles with full-text search
- `saved_items` - User bookmarks/saved items
- `conversations` - AI coaching conversations
- `messages` - Conversation messages
- `notifications` - User notifications
- `activities` - User activity timeline
- `user_settings` - User preferences

## Configuration

Configuration is managed through `backend/services/gateway/etc/api.yaml`:

```yaml
Name: api
Host: 0.0.0.0
Port: 8888

# Database configuration
DataSource: postgres://growthmind:growthmind123@localhost:5432/growthmind?sslmode=disable

# Redis configuration  
Redis:
  Host: localhost:6379
  Type: node
  Pass: ""

# JWT configuration
Auth:
  AccessSecret: growthmind-access-secret-key
  AccessExpire: 86400
  RefreshSecret: growthmind-refresh-secret-key
  RefreshExpire: 604800

# Meilisearch configuration
MeiliSearch:
  Host: http://localhost:7700
  MasterKey: growthmind123
```

## Development

### Adding New Features

1. Update API contract in `api/self-dev.api`
2. Regenerate gateway: `goctl api go -api api/self-dev.api -dir backend/services/gateway`
3. Implement business logic in `internal/logic/`
4. Add database migrations if needed
5. Update repositories and models

### Testing

```bash
make test
```

### Code Generation

The project uses go-zero code generation:

```bash
# Regenerate API handlers
goctl api go -api api/self-dev.api -dir backend/services/gateway --style gozero
```

## Current Status

✅ **Completed:**
- Project structure and setup
- Database schema and migrations
- Authentication system (JWT)
- Basic API gateway with go-zero
- User and profile management
- Development infrastructure (Docker Compose)
- Build and development tooling

🚧 **In Progress:**
- Habit tracking logic
- Goal management logic
- Article management
- Search functionality
- AI conversation system
- Notification system

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the MIT License.
