// stream.js — SSE / streaming endpoint capacity test.
//
// Holds STREAM_VUS concurrent streams open against an ai-gateway SSE endpoint
// and measures stream-open latency (time to first byte — the handler commits
// SSE headers when the stream is established), total stream duration, event
// counts, and completion rate.
//
// Auth is REQUIRED — all streaming routes sit behind the Auth middleware.
// The script aborts in setup() when no token can be acquired.
//
//   TEST_TOKEN=<jwt> k6 run stream.js
//   TEST_EMAIL=.. TEST_PASSWORD=.. k6 run stream.js
//   STREAM_VUS=20 STREAM_DURATION=5m TEST_TOKEN=<jwt> k6 run stream.js
//   STREAM_PATH=/api/v1/personalization/coaching-stream \
//     BASE_URL=http://localhost:8889 TEST_TOKEN=<jwt> k6 run stream.js
//
// Endpoint notes:
//   - /api/v1/weekly-reviews/generate-stream (default): SSE weekly review.
//     A cached review completes almost instantly; a fresh generation streams
//     thinking/delta/finalizing/complete events over tens of seconds and
//     consumes real LLM tokens. Daily per-user token quota applies — once
//     exhausted the stream emits `event: error` (HTTP status stays 200).
//   - /api/v1/personalization/coaching-stream: SSE coaching. NOTE: the prod
//     Caddyfile does not route this path to ai-gateway — through
//     api.evolella.com it lands on the gateway and 404s. Use it with a
//     direct-to-service BASE_URL (e.g. http://localhost:8889) until the
//     ingress route is added.
//
// k6 built-ins buffer SSE bodies, so per-event latency (true time-to-first-
// token) is not measurable here — stream_open_ms is the time until the
// stream is established (headers flushed). For per-event timing, use the
// k6/x/sse extension (auto-resolved by k6 ≥ v1.2) — see README.md.
//
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';
import exec from 'k6/execution';
import http from 'k6/http';
import {
  BASE_URL,
  EXPECTED_STREAM,
  acquireToken,
  countSSEvent,
  headers,
  randomIntBetween,
  streamCompleted,
  streamFailed,
} from './lib.js';

const STREAM_VUS = parseInt(__ENV.STREAM_VUS || '5', 10);
const STREAM_DURATION = __ENV.STREAM_DURATION || '2m';
// Path is normalized to include the /api/v1 prefix (it is matched against
// Caddy routes and also required when hitting ai-gateway directly).
const rawStreamPath = __ENV.STREAM_PATH || '/api/v1/weekly-reviews/generate-stream';
const STREAM_PATH =
  rawStreamPath.indexOf('/api/v1') === 0
    ? rawStreamPath
    : `/api/v1/${rawStreamPath.replace(/^\/+/, '')}`;
const STREAM_WEEK_START = __ENV.STREAM_WEEK_START || ''; // e.g. 2024-01-15
const STREAM_FORCE = __ENV.STREAM_FORCE === '1' || __ENV.STREAM_FORCE === 'true';
const STREAM_MESSAGE =
  __ENV.STREAM_MESSAGE || 'Give me one quick tip for staying consistent with my habits this week.';
// Streams are long-lived: Caddy allows 300s, keep the request timeout under it.
const STREAM_TIMEOUT = __ENV.STREAM_TIMEOUT || '240s';

export const options = {
  scenarios: {
    sse_streams: {
      executor: 'constant-vus',
      vus: STREAM_VUS,
      duration: STREAM_DURATION,
      gracefulStop: '30s',
    },
  },
  thresholds: {
    // Stream establishment should be fast; duration is LLM-bound so the bar
    // is deliberately loose — tune to your provider/model.
    stream_open_ms: ['p(95)<5000'],
    stream_duration_ms: ['p(95)<240000'],
    stream_completion_rate: ['rate>0.50'], // cached/quota-error streams pull this down; investigate if below
    stream_rejected_total: [], // reported but not gated — 429s surface here
  },
};

const streamOpenMs = new Trend('stream_open_ms', true);
const streamDurationMs = new Trend('stream_duration_ms', true);
const streamEvents = new Counter('stream_events_total');
const streamDeltas = new Counter('stream_deltas_total');
const streamErrors = new Counter('stream_error_events_total');
const streamRejected = new Counter('stream_rejected_total');
const streamCompletionRate = new Rate('stream_completion_rate');

export function setup() {
  const auth = acquireToken();
  if (!auth.token) {
    exec.test.abort(
      `stream.js requires auth — set TEST_TOKEN or TEST_EMAIL+TEST_PASSWORD ` +
        `(login status: ${auth.status || 'n/a'}, source: ${auth.source})`
    );
  }
  return auth;
}

function streamBody() {
  if (STREAM_PATH.indexOf('coaching') !== -1) {
    // GeneratePersonalizedCoachingRequest — clientMessageId makes the turn
    // idempotent across retries.
    return {
      userMessage: STREAM_MESSAGE,
      clientMessageId: `k6-stream-vu${__VU}-it${__ITER}`,
    };
  }
  // GenerateWeeklyReviewRequest — empty weekStart lets the server pick the
  // current week; forceRegenerate stays off so cached reviews short-circuit.
  const body = { forceRegenerate: STREAM_FORCE };
  if (STREAM_WEEK_START) body.weekStart = STREAM_WEEK_START;
  return body;
}

export default function (auth) {
  const res = http.post(`${BASE_URL}${STREAM_PATH}`, JSON.stringify(streamBody()), {
    headers: {
      ...headers(auth.token, true),
      Accept: 'text/event-stream',
    },
    tags: { endpoint: STREAM_PATH.replace('/api/v1/', ''), kind: 'stream' },
    responseCallback: EXPECTED_STREAM,
    timeout: STREAM_TIMEOUT,
  });

  streamOpenMs.add(res.timings.waiting); // TTFB ≈ time until SSE stream opens
  streamDurationMs.add(res.timings.duration); // request → server closes stream

  if (res.status !== 200) {
    // Pre-stream rejection: rate limit (429), auth (401), validation (400),
    // or a routing miss (404 — e.g. coaching-stream via prod Caddy).
    streamRejected.add(1);
    check(res, {
      'stream accepted (200)': (r) => r.status === 200,
    });
    sleep(randomIntBetween(2, 5));
    return;
  }

  const body = res.body || '';
  const deltas = countSSEvent(body, 'delta');
  const events =
    deltas +
    countSSEvent(body, 'thinking') +
    countSSEvent(body, 'reasoning') +
    countSSEvent(body, 'proposal') +
    countSSEvent(body, 'finalizing') +
    countSSEvent(body, 'complete') +
    countSSEvent(body, 'error');

  streamEvents.add(events);
  streamDeltas.add(deltas);
  const completed = streamCompleted(body);
  const failed = streamFailed(body);
  streamCompletionRate.add(completed);
  if (failed) streamErrors.add(1);

  check(res, {
    'stream opened (200 + event-stream)': (r) =>
      r.status === 200 && (r.headers['Content-Type'] || '').indexOf('text/event-stream') !== -1,
    'stream produced events': () => events > 0,
    'stream completed or clean-errored': () => completed || failed,
    'no error event': () => !failed,
  });

  // Brief pause between streams — a real client does not instantly reopen.
  sleep(randomIntBetween(2, 5));
}
