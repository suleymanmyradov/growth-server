# growth-server

A production-grade backend API built in **Go**, serving as the core server for a content/growth platform. Features a clean microservices-style architecture with multiple service layers.

## Features

- **Authentication** — Google OAuth 2.0 + email/password with verification and password reset
- - **Session management** — Secure session handling with token-based auth
  - - **File storage** — MinIO integration for scalable object storage
    - - **Article management** — Admin API for content creation and management
      - - **Email service** — Transactional email support (verification, reset)
        - - **Conversation & streaming** — Personalized conversation management
          - - **Database** — PostgreSQL with structured SQL migrations
           
            - ## Tech Stack
           
            - | Layer | Technology |
            - |---|---|
            - | Language | Go |
            - | Database | PostgreSQL |
            - | File Storage | MinIO |
            - | Auth | Google OAuth 2.0 |
            - | Process Management | tmux / Makefile |
            - | Linting | golangci-lint |
           
            - ## Project Structure
           
            - ```
              growth-server/
              ├── bin/        # Compiled binaries
              ├── deploy/     # Deployment configs (MinIO, tmux sessions)
              ├── pkg/        # Core packages (services, handlers, models)
              ├── services/   # Individual microservices
              ├── sql/        # Database migrations and queries
              └── Makefile    # Build and run commands
              ```

              ## Getting Started

              ```bash
              # Clone the repo
              git clone https://github.com/suleymanmyradov/growth-server.git
              cd growth-server

              # Install dependencies
              go mod download

              # Run the server
              make run
              ```

              > Requires Go 1.21+, PostgreSQL, and MinIO.
              >
              > ## Author
              >
              > **Suleyman Myradov** — [linkedin.com/in/suleyman-myradov](https://linkedin.com/in/suleyman-myradov)
