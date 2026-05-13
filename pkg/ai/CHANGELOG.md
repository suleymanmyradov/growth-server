# Changelog

All notable changes to `pkg/ai` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [0.1.0] - 2026-05-11

### Added

- `Config` with `ModelProfile` enum (cheap, cheap_long, classifier, chat, fallback), default models, cost rates, validation.
- `Client` interface with `Generate`, `Stream`, `GenerateStructured`, `RunAgent`.
- `New()` constructor with `WithHTTPClient` and `WithQuotaStore` options.
- `openRouterTransport` injects `Authorization`, `HTTP-Referer`, `X-Title` headers on every request.
- `Generate` with automatic fallback to `ModelFallback` on primary failure.
- `GenerateStructured` strips markdown code fences and unmarshals JSON.
- `Stream` with `StreamReader` adapter over Eino's `StreamReader[*schema.Message]`.
- `RunAgent` loop: model ↔ tool round-trip with configurable `MaxSteps`.
- `Tool` interface and `NewTool[I, O]` generic constructor with JSON schema reflection via `invopop/jsonschema` → `eino-contrib/jsonschema` conversion.
- `withRetry` with exponential backoff (`cenkalti/backoff/v5`) and per-model circuit breaker (`go-zero/breaker`).
- `isRetryable` detects Eino `openai.APIError` (429, 5xx) and internal `apiError`.
- Prometheus metrics: `ai_requests_total`, `ai_tokens_total`, `ai_cost_usd_total`, `ai_request_duration_seconds`.
- `QuotaStore` interface with `redisQuotaStore` (per-user daily token cap, global daily cost cap) and `noopQuotaStore`.
- Sentinel errors: `ErrQuotaExceeded`, `ErrSafetyBlock`, `ErrModelUnavailable`, `ErrInvalidProfile`, `ErrNoTools`.
- `prompts/` subpackage: typed `Template[T]` with embedded `.tmpl` files.
- `memory/` subpackage: `ConversationWindow`, `Summarizer`, `Retriever` interface stub.
- `safety/` subpackage: `Classifier` interface, `LLMClassifier` with structured JSON output.
- `aitest/` subpackage: `MockClient` with recordable responses and streams.
- Comprehensive test suite with httptest mock server.
- `README.md` with quickstart, config reference, and design decisions.
- Smoke test at `cmd/smoke/` for end-to-end validation against OpenRouter.

### Dependencies

- `github.com/cloudwego/eino v0.7.23`
- `github.com/cloudwego/eino-ext/components/model/openai v0.1.13`
- `github.com/invopop/jsonschema v0.14.0`
- `github.com/cenkalti/backoff/v5` (indirect, already in go.mod)
