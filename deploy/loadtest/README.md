# k6 load tests — Growth backend

k6 scripts for the public API at `https://api.evolella.com` (Caddy → gateway
`:8888` / ai-gateway `:8889`, all routes under `/api/v1`). Dependency-free —
only k6 built-in modules; run them as-is, no bundling or extensions needed.

| File | Purpose |
|---|---|
| `lib.js` | Shared helpers: `BASE_URL`, token acquisition, request wrappers, SSE body parsing. Not a runnable script. |
| `smoke.js` | 1-VU sanity pass over public + authenticated endpoints. Run first. |
| `load.js` | Realistic mixed traffic, ramp 0→~25→`MAX_VUS` (default 50) over ~10 min. |
| `stress.js` | Ramping request-rate until failure, with abort thresholds. Read-only. |
| `stream.js` | Holds N concurrent SSE streams open; measures stream-open latency, duration, event counts, completion. Auth required. |

## Install k6

```bash
# macOS
brew install k6

# Docker (no install)
docker run --rm -i grafana/k6 run - <smoke.js   # note: './lib.js' imports need a volume mount:
docker run --rm -i -v "$PWD:/scripts" -w /scripts grafana/k6 run smoke.js

# Other platforms: https://grafana.com/docs/k6/latest/set-up/install-k6/
```

## Configuration

All knobs are environment variables — nothing is hardcoded.

| Var | Default | Used by | Meaning |
|---|---|---|---|
| `BASE_URL` | `https://api.evolella.com` | all | Public origin (or `http://localhost:8888` / `:8889` to bypass Caddy) |
| `TEST_TOKEN` | — | all | Pre-minted `accessToken` (JWT). Wins over credentials. |
| `TEST_EMAIL` + `TEST_PASSWORD` | — | all | Test-user creds → one `POST /auth/login` in `setup()` |
| `DEVICE_ID` | `k6-loadtest` | auth | Optional `X-Device-Id` header on login |
| `MAX_VUS` | `50` | load.js | Peak VUs |
| `STRESS_TARGET_RPS` | `120` | stress.js | Peak request rate (req/s) |
| `STREAM_VUS` | `5` | stream.js | Concurrent open streams |
| `STREAM_DURATION` | `2m` | stream.js | How long the scenario runs |
| `STREAM_PATH` | `/api/v1/weekly-reviews/generate-stream` | stream.js | SSE endpoint path |
| `STREAM_WEEK_START` | server default | stream.js | `weekStart` for weekly-review generation |
| `STREAM_FORCE` | off | stream.js | `1`/`true` → `forceRegenerate` (burns LLM tokens every request!) |
| `STREAM_MESSAGE` | a short prompt | stream.js | `userMessage` for coaching-stream |
| `STREAM_TIMEOUT` | `240s` | stream.js | Per-request timeout (Caddy caps streams at 300s) |

Without credentials, scripts run **public traffic only**. With a token or
creds, `load.js`/`stress.js` add authenticated reads (and a small share of
self-cleaning writes in `load.js`); `stream.js` requires auth and aborts
without it.

## Running

```bash
cd deploy/loadtest

# 1. Sanity — should fully pass before anything else
k6 run smoke.js
TEST_TOKEN=$TOKEN k6 run smoke.js

# 2. Load — ~10 min, 0→50 VUs, mixed read/write
TEST_TOKEN=$TOKEN k6 run load.js
MAX_VUS=100 TEST_TOKEN=$TOKEN k6 run load.js

# 3. Stress — ramps req/s until failure thresholds abort the run
k6 run stress.js
STRESS_TARGET_RPS=300 TEST_TOKEN=$TOKEN k6 run stress.js

# 4. Streaming — 5 concurrent SSE streams for 2m (auth required)
TEST_TOKEN=$TOKEN k6 run stream.js
STREAM_VUS=20 STREAM_DURATION=5m TEST_TOKEN=$TOKEN k6 run stream.js
```

Useful flags: `--out influxdb=...`/`--out json=result.json` for metrics export,
`-e KEY=VAL` instead of shell env (`k6 run -e TEST_TOKEN=... load.js`),
`--summary-export summary.json` for a machine-readable summary.

## Endpoint coverage

Public (no auth): `GET /health` (Caddy edge), `GET /api/v1/articles`,
`GET /api/v1/articles/:id`, `GET /api/v1/articles/featured`,
`GET /api/v1/categories`, `GET /api/v1/site-settings`,
`GET /api/v1/habit-templates`, `GET /api/v1/goal-templates`,
`GET /api/v1/search`.

Authenticated (gateway `:8888`): `GET /profile/me`, `GET /habits`,
`POST /habits` + `DELETE /habits/:id` (create-then-delete, self-cleaning),
`GET /goals`, `GET /check-ins/today`, `GET /check-ins/checked-today`,
`POST /check-ins`, `GET /notifications`, `GET /notifications/unread-count`,
`GET /activity`, `GET /settings`, `PUT /settings`, `GET /saved`,
`GET /weekly-reviews`, `GET /weekly-reviews/current`,
`GET /billing/overview`, `GET /personalization/coaching-profile`.

Authenticated (ai-gateway `:8889`): `GET /conversations`, `GET /memory/facts`,
`POST /weekly-reviews/generate-stream` (SSE), optionally
`POST /personalization/coaching-stream` (SSE — see caveat below).

Deliberately **not** exercised: `POST /auth/register`, `/forgot-password`,
`/resend-verification` (send real emails), `POST /auth/logout` (kills the
shared test session), all `/billing/paddle-checkout|portal|paddle-webhook` (Paddle
side effects), `POST /profile/export`, `DELETE /profile`, device
registration, voice/multipart endpoints.

## Interpreting results

k6 prints live metrics plus an end-of-run summary; a **failed threshold makes
the exit code non-zero**, so these scripts are CI-friendly.

- `http_req_duration{kind=read|write|stream}` — per-class latency. The suite's
  headline gate: **reads p95 < 500 ms** at 50 VUs.
- `http_req_failed` — rate of non-expected responses. 429s and whitelisted
  404s are excluded via `responseCallback` and surfaced separately instead.
- `rate_limited_429` (stress.js) — a spike here means you hit the rate
  limiter, not the service capacity. Distinguish the two before drawing
  conclusions.
- `server_errors_5xx` — real breakage signal.
- `checks` — functional assertions per endpoint (shape + status), should stay
  ≈100%.
- stream.js adds: `stream_open_ms` (time until the SSE stream is established —
  headers flush; **not** time-to-first-token), `stream_duration_ms`,
  `stream_events_total`, `stream_deltas_total`, `stream_error_events_total`
  (quota/upstream failures — HTTP stays 200, the `error` event carries it),
  `stream_completion_rate`, `stream_rejected_total` (pre-stream 429/401/404s).

### Suggested capacity targets

| Scenario | Target |
|---|---|
| Baseline load | 50 concurrent users; reads p95 < 500 ms, p99 < 1.5 s; error rate < 1% |
| Smoke | all checks pass on a healthy deploy |
| Stress | locate the knee: rate where `server_errors_5xx` starts or read p95 crosses 3 s. Re-run after infra changes to detect regressions. |
| Streaming | `STREAM_VUS` streams sustained for `STREAM_DURATION` without connection drops; `stream_open_ms` p95 < 5 s; `stream_completion_rate` high when below the AI/token quota |

Tune thresholds in each script's `options` as the service evolves.

## Rate limits & quotas (production values)

From `deploy/config/*.yaml` — your results **will** hit these:

| Bucket | Limit | Covers |
|---|---|---|
| auth | 10 req / 60 s / IP | all `/api/v1/auth/*` → token is acquired **once** in `setup()` |
| AI | 10 req / 60 s / user | AI-generation POSTs: `POST /check-ins`, `/conversations/:id/messages`, `/weekly-reviews/generate(-stream)`, `/personalization/coaching(-stream)`, `/personalization/onboarding-habits`, `/personalization/voice-turn` |
| search | 30 req / 60 s / IP | `GET /api/v1/search` |
| daily token quota | per-plan, per-user | AI generation — streams return `event: error` when exhausted |

429s are expected under load — the scripts whitelist them and count them
separately. A weekly-review stream that finds a cached review returns a fast
`complete` event; fresh generations are the slow path.

## ⚠️ Running against prod (`api.evolella.com`)

These scripts are safe to run **gently**, but read this first:

- **Use a dedicated test account** — `load.js` writes mutate it (settings
  overwritten to dark/UTC/09:00; habits created+deleted; real check-ins posted).
- **Writes have side effects**: every `POST /check-ins` enqueues async AI
  feedback work; every weekly-review generation spends LLM tokens against the
  daily quota. `STREAM_FORCE=1` regenerates on every request — expensive.
- **Never** hit `auth/register` or `auth/forgot-password` in a loop — they send
  real emails. The scripts deliberately exclude them.
- **Gentle rates only, off-peak**: for prod keep `MAX_VUS ≤ 10`, `STREAM_VUS ≤ 3`,
  and skip `stress.js` entirely (or `STRESS_TARGET_RPS ≤ 10`). Run the heavy
  profiles against a staging/perf environment or locally via `docker-compose`.
- One IP does the load: per-IP buckets (auth, search) will throttle a single
  load generator earlier than real distributed traffic would.
- To bypass Caddy and hit a service directly: `BASE_URL=http://localhost:8888`
  (gateway) or `http://localhost:8889` (ai-gateway).

## SSE measurement notes

k6's built-in `http` buffers the whole response, so a stream request blocks
until the server closes it (or `STREAM_TIMEOUT` hits at 240 s; Caddy's proxy
read timeout is 300 s). What is measured:

- `stream_open_ms` = `timings.waiting` — time until SSE headers flush (the
  handler commits `200 + text/event-stream` when the stream is established).
- `stream_duration_ms`, event counts, completion — parsed from the buffered
  body after close.
- **True time-to-first-token is not measurable** this way. For per-event
  timing use the `k6/x/sse` extension — since k6 v1.2 it auto-resolves, no
  custom build needed:

```js
import sse from 'k6/x/sse';
// sse.open(url, {method:'POST', headers: {...}, body: JSON.stringify(...)},
//   (client) => client.on('event', (e) => /* e.name = thinking|delta|complete|error */));
```

### Known caveat: `coaching-stream` ingress route

`POST /api/v1/personalization/coaching-stream` is registered on ai-gateway
(`aiapi.go`), but **`deploy/caddy/Caddyfile` does not route it** — it isn't in
`@ai_exact` or `@ai_prefix`, so through `api.evolella.com` it falls through to
the gateway and returns 404. `stream.js` therefore defaults to
`weekly-reviews/generate-stream`. To exercise coaching-stream, add the path to
the Caddyfile `@ai_exact` block, or point `STREAM_PATH` at it with a
direct-to-service `BASE_URL=http://localhost:8889`.
