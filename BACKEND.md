# GrowthMind Backend

## Overview

This document describes the planned backend for GrowthMind. It defines the tech stack, API contract location, service layout, local development workflow, and deployment approach. It is backend-only and does not alter the frontend README.

## Tech Stack

- Language/Runtime: Go 1.22+
- Framework/Tooling: go-zero (REST gateway + zRPC), goctl for code generation
- Contracts: go-zero API DSL for HTTP, Protocol Buffers for RPC (future)
- Storage/Infra (minimum): Postgres, Redis, Meilisearch
- Optional Infra (later): Kafka + ClickHouse (analytics), MinIO (object storage), Jaeger (tracing)
- Auth: JWT

## API Contract

- Path: `api/self-dev.api` (go-zero API DSL)
- Domains: auth, profile, habits, goals, articles (public), saved, conversations (coach/therapist), search, notifications, activity, settings, report
- Pagination: `PageReq` + `PageResp[T]`
- Security: JWT for user-specific domains

> Action item: add `api/self-dev.api` to the repository and generate gateway/server scaffolding with goctl.

## Repository Layout (planned)

```
api/
  self-dev.api          # HTTP API contract
backend/
  services/
    gateway/            # REST gateway (go-zero)
      api/contract/     # copy of API DSL for gateway
      internal/...      # generated + custom code
    auth/               # zRPC services (as added)
    profile/
    habits/
    goals/
    articles/
    search/
    notifications/
    activity/
    conversations/
  deployment/
    dependencies/       # docker-compose for Postgres, Redis, Meilisearch (+ optional stack)
    docker/             # per-service Dockerfiles
DB/
  migrations/           # SQL migrations
```

## Local Development

Prereqs: Go toolchain, Docker, goctl, (optional) golang-migrate.

1) Start minimal infrastructure (Postgres, Redis, Meilisearch):

```bash
docker compose -f backend/deployment/dependencies/docker-compose.yml up -d
```

2) Generate REST gateway from API contract:

```bash
# requires goctl installed
# https://go-zero.dev/en/docs/tasks/api/overview/
goctl api go -api api/self-dev.api -dir backend/services/gateway
```

3) Run services locally:

```bash
# example: gateway
make -C backend run-gateway
# add per-domain run targets as services are implemented
```

4) Database migrations (example):

```bash
migrate -path DB/migrations -database "$DATABASE_URL" up
```

## Security

- JWT auth for protected endpoints.
- Rate limit and request size checks on sensitive routes.
- Secrets via environment variables/secret store (never commit secrets).

## Integration Map (Frontend ↔ Backend)

- Profile: `/v1/profile/me`, `PUT /v1/profile`
- Habits: CRUD + toggle + reset
- Goals: CRUD + toggle
- Articles: list/get (public)
- Saved: list/save/remove
- Conversations (coach/therapist): list/start/send
- Search: `/v1/search?q=...`
- Notifications: list/mark-read
- Activity: paginated timeline
- Settings: preferences (theme/notifications)
- Report: submit feedback/abuse

## Next Steps

- Add `api/self-dev.api` with the full contract.
- Run `goctl api go` to scaffold the gateway.
- Create DB schemas/migrations for auth, profile, habits, goals.
- Implement auth flows (register/login/refresh), configure JWT secret.
- Implement domain services and wire to gateway.
- Prepare `backend/deployment/dependencies/docker-compose.yml` for local infra.
- Add CI to build/push images and run migrations on deploy.
