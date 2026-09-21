// stress.js — find the breaking point of the Growth API.
//
// Ramps the request rate (not just VUs) until the service fails. Uses the
// ramping-arrival-rate executor so the target load keeps growing even when
// the server slows down — that is what actually saturates the backend.
//
// Aborts automatically when either failure threshold trips:
//   - >10% of requests failing for 30s straight, or
//   - p95 read latency above 3s for 30s straight.
// The stage where it aborts is roughly your breaking point.
//
//   k6 run stress.js                          # ramp to ~120 req/s
//   STRESS_TARGET_RPS=300 k6 run stress.js    # push harder
//   TEST_TOKEN=<jwt> k6 run stress.js         # include authenticated reads
//
// READ-ONLY by design: stress runs perform no writes, so no data is created.
//
import { check } from 'k6';
import { Counter } from 'k6/metrics';
import {
  EXPECTED_READ_OR_404,
  acquireToken,
  apiGet,
  pick,
  pickWeighted,
} from './lib.js';

// Total target request rate at peak (requests per second).
const TARGET_RPS = parseInt(__ENV.STRESS_TARGET_RPS || '120', 10);

const rateLimited = new Counter('rate_limited_429');
const serverErrors = new Counter('server_errors_5xx');

export const options = {
  scenarios: {
    stress: {
      executor: 'ramping-arrival-rate',
      startRate: 5,
      timeUnit: '1s',
      preAllocatedVUs: 50,
      maxVUs: 1000,
      stages: [
        { duration: '2m', target: Math.max(1, Math.round(TARGET_RPS * 0.17)) }, // ~20%
        { duration: '3m', target: Math.max(1, Math.round(TARGET_RPS * 0.33)) }, // ~33%
        { duration: '3m', target: Math.max(1, Math.round(TARGET_RPS * 0.67)) }, // ~67%
        { duration: '4m', target: TARGET_RPS }, // 100% — expected to break here
        { duration: '2m', target: 0 }, // drain
      ],
    },
  },
  thresholds: {
    // Abort conditions — sustained failure or collapse-level latency.
    http_req_failed: [
      { threshold: 'rate<0.10', abortOnFail: true, delayAbortEval: '30s' },
    ],
    'http_req_duration{kind:read}': [
      { threshold: 'p(95)<3000', abortOnFail: true, delayAbortEval: '30s' },
    ],
    server_errors_5xx: [],
    rate_limited_429: [],
  },
};

export function setup() {
  const auth = acquireToken();
  if (!auth.token) {
    console.log('i no credentials provided — stressing public endpoints only');
  }
  return auth;
}

// ---------------------------------------------------------------------------
// Read-only action mix — same shape as load.js minus writes and minus the
// expensive flows. Every action is a single HTTP request so iteration rate
// maps directly to request rate.
// ---------------------------------------------------------------------------

const SEARCH_TERMS = ['habit', 'goal', 'focus', 'health'];

function buildActions(authed) {
  const actions = [
    { weight: 25, name: 'articles_list', run: () => apiGet('/articles?page=1&limit=10', '', { endpoint: 'articles_list' }) },
    { weight: 10, name: 'categories', run: () => apiGet('/categories?entityType=habit', '', { endpoint: 'categories' }) },
    { weight: 8, name: 'site_settings', run: () => apiGet('/site-settings', '', { endpoint: 'site_settings' }) },
    { weight: 8, name: 'habit_templates', run: () => apiGet('/habit-templates', '', { endpoint: 'habit_templates' }) },
    { weight: 8, name: 'goal_templates', run: () => apiGet('/goal-templates', '', { endpoint: 'goal_templates' }) },
    // Search is per-IP rate-limited — a modest share still exercises Meilisearch.
    { weight: 4, name: 'search', run: () => apiGet(`/search?q=${pick(SEARCH_TERMS)}&page=1&limit=10`, '', { endpoint: 'search' }) },
  ];
  if (authed) {
    actions.push(
      { weight: 6, name: 'profile_me', run: (t) => apiGet('/profile/me', t, { endpoint: 'profile_me' }) },
      { weight: 8, name: 'habits_list', run: (t) => apiGet('/habits?page=1&limit=20', t, { endpoint: 'habits_list' }) },
      { weight: 6, name: 'goals_list', run: (t) => apiGet('/goals?page=1&limit=20', t, { endpoint: 'goals_list' }) },
      { weight: 4, name: 'checkins_today', run: (t) => apiGet('/check-ins/today', t, { endpoint: 'checkins_today' }) },
      { weight: 4, name: 'unread_count', run: (t) => apiGet('/notifications/unread-count', t, { endpoint: 'unread_count' }) },
      { weight: 4, name: 'settings_get', run: (t) => apiGet('/settings', t, { endpoint: 'settings_get' }) },
      { weight: 3, name: 'activity', run: (t) => apiGet('/activity?page=1&limit=20', t, { endpoint: 'activity' }) },
      { weight: 3, name: 'conversations', run: (t) => apiGet('/conversations?page=1&limit=20', t, { endpoint: 'conversations' }) },
      {
        weight: 2,
        name: 'weekly_review_current',
        run: (t) => apiGet('/weekly-reviews/current', t, {
          endpoint: 'weekly_review_current',
          expected: EXPECTED_READ_OR_404,
        }),
      }
    );
  }
  return actions;
}

let actions = null;

export default function (auth) {
  if (actions === null) {
    actions = buildActions(!!auth.token);
  }
  const res = pickWeighted(actions).run(auth.token);
  if (res.status === 429) {
    rateLimited.add(1);
  } else if (res.status >= 500) {
    serverErrors.add(1);
  }
  check(res, { 'status under 500': (r) => r.status < 500 });
}
