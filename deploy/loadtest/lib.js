// lib.js — shared helpers for the Growth backend k6 load-test suite.
//
// Dependency-free: only k6 built-in modules. Imported by smoke.js, load.js,
// stress.js and stream.js. See README.md for usage.
import http from 'k6/http';

// ---------------------------------------------------------------------------
// Configuration (all overridable via environment variables)
// ---------------------------------------------------------------------------

// Public origin. All API routes live under /api/v1 behind Caddy.
export const BASE_URL = (__ENV.BASE_URL || 'https://api.evolella.com').replace(/\/+$/, '');
export const API_BASE = `${BASE_URL}/api/v1`;

// Credentials — never hardcoded. Either inject a ready-made access token, or
// provide test-user credentials and lib will log in once in setup().
export const TEST_TOKEN = __ENV.TEST_TOKEN || '';
export const TEST_EMAIL = __ENV.TEST_EMAIL || '';
export const TEST_PASSWORD = __ENV.TEST_PASSWORD || '';

// Optional X-Device-Id header for auth calls (the API accepts it optionally).
export const DEVICE_ID = __ENV.DEVICE_ID || 'k6-loadtest';

// ---------------------------------------------------------------------------
// Expected-status presets
//
// k6 counts every response outside 200–399 as a failed request. Rate limiting
// (429) and a couple of legitimate 404s are expected outcomes under load, so
// they are whitelisted here and tracked via custom metrics instead of tripping
// http_req_failed.
// ---------------------------------------------------------------------------
export const EXPECTED_READ = http.expectedStatuses({ min: 200, max: 399 });
// For resources that may legitimately not exist (featured article, current
// weekly review).
export const EXPECTED_READ_OR_404 = http.expectedStatuses({ min: 200, max: 399 }, 404);
// Writes may be rejected by per-user/per-IP rate limits (429) or business
// rules (e.g. duplicate check-in → 4xx).
export const EXPECTED_WRITE = http.expectedStatuses({ min: 200, max: 399 }, 429);
// SSE streams can be rejected pre-flight by rate limits (429).
export const EXPECTED_STREAM = http.expectedStatuses({ min: 200, max: 399 }, 429);

// ---------------------------------------------------------------------------
// Header / request helpers
// ---------------------------------------------------------------------------

export function headers(token, hasBody) {
  const h = { Accept: 'application/json' };
  if (hasBody) h['Content-Type'] = 'application/json';
  if (token) h.Authorization = `Bearer ${token}`;
  return h;
}

export function apiGet(path, token, opts) {
  const o = opts || {};
  return http.get(`${API_BASE}${path}`, {
    headers: headers(token, false),
    tags: { endpoint: o.endpoint || path, kind: o.kind || 'read' },
    responseCallback: o.expected || EXPECTED_READ,
    timeout: o.timeout || '30s',
  });
}

export function apiPost(path, body, token, opts) {
  const o = opts || {};
  return http.post(`${API_BASE}${path}`, JSON.stringify(body), {
    headers: headers(token, true),
    tags: { endpoint: o.endpoint || path, kind: o.kind || 'write' },
    responseCallback: o.expected || EXPECTED_WRITE,
    timeout: o.timeout || '30s',
  });
}

export function apiPut(path, body, token, opts) {
  const o = opts || {};
  return http.put(`${API_BASE}${path}`, JSON.stringify(body), {
    headers: headers(token, true),
    tags: { endpoint: o.endpoint || path, kind: o.kind || 'write' },
    responseCallback: o.expected || EXPECTED_WRITE,
    timeout: o.timeout || '30s',
  });
}

export function apiDel(path, token, opts) {
  const o = opts || {};
  return http.del(`${API_BASE}${path}`, null, {
    headers: headers(token, false),
    tags: { endpoint: o.endpoint || path, kind: o.kind || 'write' },
    responseCallback: o.expected || EXPECTED_WRITE,
    timeout: o.timeout || '30s',
  });
}

// ---------------------------------------------------------------------------
// Auth
// ---------------------------------------------------------------------------

// Acquires an access token. Call this from setup() so it runs exactly once per
// test — the auth endpoints are rate-limited at ~10 req/min per IP, so logging
// in per VU or per iteration would 429 almost immediately.
//
// Order of precedence:
//   1. TEST_TOKEN env var (pre-minted access token)
//   2. TEST_EMAIL + TEST_PASSWORD env vars → POST /api/v1/auth/login
//   3. no token (public-traffic-only mode)
//
// Returns { token, source, status } — token is '' when unavailable.
export function acquireToken() {
  if (TEST_TOKEN) {
    return { token: TEST_TOKEN, source: 'TEST_TOKEN', status: 0 };
  }
  if (!TEST_EMAIL || !TEST_PASSWORD) {
    return { token: '', source: 'none', status: 0 };
  }
  const res = http.post(
    `${API_BASE}/auth/login`,
    JSON.stringify({ email: TEST_EMAIL, password: TEST_PASSWORD }),
    {
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
        'X-Device-Id': DEVICE_ID,
      },
      tags: { endpoint: 'auth_login', kind: 'auth' },
      responseCallback: http.expectedStatuses(200),
      timeout: '15s',
    }
  );
  // AuthResponse is NOT wrapped in the standard {data: ...} envelope —
  // accessToken sits at the top level.
  const body = safeJson(res);
  if (res.status === 200 && body && body.accessToken) {
    return { token: body.accessToken, source: 'login', status: res.status };
  }
  return {
    token: '',
    source: 'login-failed',
    status: res.status,
    error: (body && (body.msg || body.message)) || res.body || '',
  };
}

// ---------------------------------------------------------------------------
// Parsing / randomness
// ---------------------------------------------------------------------------

// res.json() throws on non-JSON bodies; never let a bad body kill an iteration.
export function safeJson(res) {
  try {
    return res.json();
  } catch (e) {
    return null;
  }
}

export function randomIntBetween(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

export function pick(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

// Weighted pick over [{ weight, ... }] entries. Weights are relative — they
// do not need to sum to 1 or 100.
export function pickWeighted(actions) {
  let total = 0;
  for (const a of actions) total += a.weight;
  let r = Math.random() * total;
  for (const a of actions) {
    r -= a.weight;
    if (r <= 0) return a;
  }
  return actions[actions.length - 1];
}

// ---------------------------------------------------------------------------
// SSE helpers (buffered-body analysis)
//
// k6's built-in http module buffers the whole response, so a request to an
// SSE endpoint blocks until the server closes the stream (or the request
// timeout hits). Once it returns we can still analyze the complete event
// stream that arrived on the wire: events look like "event: <name>\ndata:
// <json>\n\n" and keepalives are ": keepalive" comment lines.
// ---------------------------------------------------------------------------

// Counts "event: <name>" occurrences in an SSE body.
export function countSSEvent(body, name) {
  if (!body) return 0;
  const needle = `event: ${name}\n`;
  let count = 0;
  let idx = 0;
  while ((idx = body.indexOf(needle, idx)) !== -1) {
    count++;
    idx += needle.length;
  }
  return count;
}

// True when the SSE body terminated with `event: complete` (success path).
export function streamCompleted(body) {
  return !!body && body.indexOf('event: complete\n') !== -1;
}

// True when the SSE body contains `event: error` — note the HTTP status is
// still 200 in that case, so this must be checked on the body, not the status.
export function streamFailed(body) {
  return !!body && body.indexOf('event: error\n') !== -1;
}
