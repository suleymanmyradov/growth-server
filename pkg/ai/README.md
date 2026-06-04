# pkg/ai — Shared AI/LLM Layer

Shared AI/LLM layer for the Growth platform. Used by coach,
notifications, and agent microservices. Built on [Eino](https://github.com/cloudwego/eino)
with the OpenAI-compatible adapter routed through [OpenRouter](https://openrouter.ai).

## Quickstart

### 1. Feedback Generation (Phase 3C)

```go
import ai "github.com/suleymanmyradov/growth-server/pkg/ai"

cfg := ai.Config{APIKey: os.Getenv("OPENROUTER_API_KEY")}
client, err := ai.New(cfg)
if err != nil { /* handle */ }

resp, err := client.Generate(ctx, ai.GenerateRequest{
    ModelProfile: ai.ModelCheap,
    System:       "You are a supportive accountability coach.",
    Messages: []ai.Message{
        {Role: ai.RoleUser, Content: "I completed my daily coding goal!"},
    },
    Metadata: ai.Metadata{UserID: "user123", Feature: "check-in-feedback"},
})
```

### 2. Streaming Chat (Phase 5)

```go
sr, err := client.Stream(ctx, ai.GenerateRequest{
    ModelProfile: ai.ModelChat,
    System:       "You are a conversational coach.",
    Messages:     history,
})
if err != nil { /* handle */ }
defer sr.Close()

for {
    chunk, err := sr.Recv()
    if err == io.EOF { break }
    if err != nil { /* handle */ }
    fmt.Print(chunk.Delta)
}
```

### 3. Agent with Tools (Phase 5)

```go
searchTool := ai.NewTool[SearchInput, SearchOutput](ai.ToolSpec{
    Name:        "search_activities",
    Description: "Search user activities by date range.",
    Handler:     searchHandler,
})

resp, err := client.RunAgent(ctx, ai.AgentRequest{
    ModelProfile: ai.ModelChat,
    Messages:     history,
    Tools:        []ai.Tool{searchTool},
    MaxSteps:     10,
})
```

## Config Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `api_key` | string | **required** | OpenRouter API key (env: `OPENROUTER_API_KEY`) |
| `base_url` | string | `https://openrouter.ai/api/v1` | API base URL |
| `models` | map | see DefaultModels | ModelProfile → model ID mapping |
| `default_timeout` | duration | `30s` | Per-request timeout |
| `max_retries` | int | `3` | Max retries for 429/5xx |
| `retry_backoff` | duration | `500ms` | Initial retry backoff |
| `http_referer` | string | `https://growth.app` | OpenRouter analytics header |
| `x_title` | string | `growth` | OpenRouter analytics header |
| `fallback_policy.enabled` | bool | `false` | Enable fallback to ModelFallback |
| `fallback_policy.max_failures` | int | `2` | Failures before fallback |
| `quota.redis_addr` | string | | Redis address for quota tracking |
| `quota.user_daily_token_cap` | int64 | `0` | Per-user daily token cap (0=unlimited) |
| `quota.global_daily_cost_cap_usd` | float64 | `0` | Global daily spend cap (0=unlimited) |

### Model Profiles

| Profile | Default Model | Use Case |
|---------|--------------|----------|
| `cheap` | `deepseek/deepseek-chat-v3` | High-volume, low-latency feedback |
| `cheap_long` | `google/gemini-2.0-flash-001` | Longer structured generation |
| `classifier` | `google/gemini-2.0-flash-lite-001` | Safety classification |
| `chat` | `openai/gpt-4o-mini` | Conversational multi-turn chat |
| `fallback` | `anthropic/claude-3.5-haiku` | Quality fallback |

### Go-Zero YAML Example

```yaml
AI:
  ApiKey: "${OPENROUTER_API_KEY}"
  BaseURL: "https://openrouter.ai/api/v1"
  DefaultTimeout: 30s
  MaxRetries: 3
  RetryBackoff: 500ms
  HttpReferer: "https://growth.app"
  XTitle: "growth"
  FallbackPolicy:
    Enabled: true
    MaxFailures: 2
  Quota:
    RedisAddr: "localhost:6379"
    UserDailyTokenCap: 100000
    GlobalDailyCostCapUsd: 5.0
```

## Subpackages

- **prompts/** — Typed Go `text/template` loader with embedded `.tmpl` files
- **memory/** — `ConversationWindow`, `Summarizer`, `Retriever` interface stub
- **safety/** — `Classifier` interface and `LLMClassifier` implementation
- **aitest/** — `MockClient` for downstream service tests

## Observability

- **Logging**: Every call logged via `logx.WithContext(ctx)` with structured fields (profile, model, feature, user_id, conversation_id, prompt_tokens, completion_tokens, latency_ms, cost_usd). Logs propagate trace_id/span_id from upstream tracing middleware. **Never** logs prompt/completion contents at info level.
- **Metrics**: Prometheus counters (`ai_requests_total`, `ai_tokens_total`, `ai_cost_usd_total`) and histogram (`ai_request_duration_seconds`).

## Resilience

- **Retry**: Exponential backoff via `cenkalti/backoff/v5` with configurable max retries
- **Circuit Breaker**: Per-model `go-zero/breaker` — opens after consecutive failures
- **Fallback**: Automatic retry with `ModelFallback` when primary fails

## Cost Guardrails

- **Per-user daily token cap**: Redis-backed, returns `ErrQuotaExceeded` when exceeded
- **Global daily spend cap**: Redis-backed, tracked in microdollars
- **Fail-open**: If Redis is unavailable, calls proceed (with error log)

## Testing

```bash
# Run all tests
go test ./pkg/ai/...

# Run with coverage
go test ./pkg/ai/... -cover

# Run specific test
go test ./pkg/ai/ -run TestClient_Generate -v
```

### Mock Client for Downstream Services

```go
import "github.com/suleymanmyradov/growth-server/pkg/ai/aitest"

mc := aitest.NewMockClient()
mc.RecordResponse(ai.ModelCheap, ai.Message{
    Role: ai.RoleAssistant, Content: "Great work!",
}, ai.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}, 0.0001)

// Use mc anywhere ai.Client is expected
```

## Design Decisions

- **Framework independence**: Our `Message`, `Tool`, `ToolCall` types mirror Eino's but live in `pkg/ai`. Eino is an implementation detail.
- **No product-specific prompts**: Only the minimal `example.v1.tmpl` lives here. Business prompts belong in each microservice.
- **No vector DB**: The `Retriever` interface is a stub. When vector storage is needed, implement it in the owning microservice.
- **No direct OpenAI calls**: All traffic goes through OpenRouter via the Eino OpenAI adapter.

## Prompt Versioning Policy

Prompt templates in `prompts/` follow a **semantic versioning** convention embedded in the filename:

```
<name>.v<major>.tmpl
```

**Rules:**

1. **Never edit a published version.** Any wording change — even a single word — must create a new version (bump the major number).
2. **Consuming services pin to a specific version.** They call `LoadTemplate[T](fs, "feedback.v2.tmpl")` and never auto-upgrade.
3. **Old versions are never deleted.** They remain in the embedded FS for backward compatibility. Mark deprecated versions with a comment at the top of the `.tmpl` file.
4. **Version zero (`v0`) is pre-release.** Templates named `*.v0.tmpl` may be edited freely and should not be used in production.

**Rationale:** LLM output is highly sensitive to prompt wording. A single word change can alter tone, safety, or structured output format. Version-locking ensures reproducible behavior across deployments.

## Metadata Contract

Every call to `Client.Generate`, `Stream`, or `RunAgent` must include a `Metadata` struct:

```go
type Metadata struct {
    UserID         string `json:"user_id,omitempty"`
    Feature        string `json:"feature,omitempty"`
    ConversationID string `json:"conversation_id,omitempty"`
}
```

| Field | Required | Purpose |
|-------|----------|---------|
| `user_id` | **Yes** | Per-user quota tracking, cost attribution, log correlation. Omitting this silently disables user-level quota enforcement. |
| `feature` | **Yes** | Metric label (`ai_requests_total{feature=...}`), cost breakdown by feature (e.g. `check-in-feedback`, `weekly-review`, `coach-chat`). Omitting this makes feature-level observability impossible. |
| `conversation_id` | Recommended | Links multi-turn messages in logs. Not used for quota or metrics, but critical for debugging conversational flows. |

**If a microservice omits `user_id` or `feature`, observability silently degrades.** There is no runtime enforcement — the contract is by convention. Code reviewers should reject calls that omit required fields.
