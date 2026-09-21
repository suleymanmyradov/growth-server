// smoke.js — 1-VU sanity pass over the Growth API.
//
// Verifies the edge (Caddy /health), public read endpoints, and — when a
// token or credentials are provided — a representative set of authenticated
// endpoints. Run this first; it should fully pass before any load test.
//
//   k6 run smoke.js
//   TEST_TOKEN=<jwt> k6 run smoke.js
//   TEST_EMAIL=<email> TEST_PASSWORD=<pass> k6 run smoke.js
//   BASE_URL=http://localhost:8888 k6 run smoke.js   # direct to gateway
//
import { check, group, sleep } from 'k6';
import http from 'k6/http';
import {
  BASE_URL,
  EXPECTED_READ,
  EXPECTED_READ_OR_404,
  acquireToken,
  apiGet,
  safeJson,
} from './lib.js';

export const options = {
  vus: 1,
  iterations: 1,
  thresholds: {
    checks: ['rate>0.95'],
    http_req_failed: ['rate<0.05'],
    http_req_duration: ['p(95)<2000'],
  },
};

export function setup() {
  return acquireToken();
}

export default function (auth) {
  const token = auth.token;

  // --- Edge / infra -------------------------------------------------------
  group('edge', () => {
    // /health is served by Caddy itself — proves TLS + the proxy are up,
    // but says nothing about the gateway behind it.
    const res = http.get(`${BASE_URL}/health`, {
      tags: { endpoint: 'edge_health', kind: 'read' },
      responseCallback: EXPECTED_READ,
    });
    check(res, {
      'edge /health is 200': (r) => r.status === 200,
      'edge /health body is ok': (r) => (r.body || '').trim() === 'ok',
    });
  });

  // --- Public endpoints (no auth) -----------------------------------------
  group('public', () => {
    const articles = apiGet('/articles?page=1&limit=5', '', { endpoint: 'articles_list' });
    check(articles, {
      'GET /articles 200': (r) => r.status === 200,
      'GET /articles returns data[] + page': (r) => {
        const b = safeJson(r);
        return b && Array.isArray(b.data) && b.page !== undefined;
      },
    });

    const featured = apiGet('/articles/featured', '', {
      endpoint: 'articles_featured',
      expected: EXPECTED_READ_OR_404, // 404 is valid when nothing is featured
    });
    check(featured, {
      'GET /articles/featured 200 or 404': (r) => r.status === 200 || r.status === 404,
    });

    const categories = apiGet('/categories?entityType=habit', '', { endpoint: 'categories' });
    check(categories, {
      'GET /categories 200': (r) => r.status === 200,
      'GET /categories returns data[]': (r) => {
        const b = safeJson(r);
        return b && Array.isArray(b.data);
      },
    });

    const settings = apiGet('/site-settings', '', { endpoint: 'site_settings' });
    check(settings, { 'GET /site-settings 200': (r) => r.status === 200 });

    const habitTemplates = apiGet('/habit-templates', '', { endpoint: 'habit_templates' });
    check(habitTemplates, { 'GET /habit-templates 200': (r) => r.status === 200 });

    const goalTemplates = apiGet('/goal-templates', '', { endpoint: 'goal_templates' });
    check(goalTemplates, { 'GET /goal-templates 200': (r) => r.status === 200 });

    // Public search — per-IP rate limited (~30/min); one call is safe.
    const search = apiGet('/search?q=habit&type=article&page=1&limit=5', '', { endpoint: 'search' });
    check(search, {
      'GET /search 200': (r) => r.status === 200,
      'GET /search returns data[]': (r) => {
        const b = safeJson(r);
        return b && Array.isArray(b.data);
      },
    });
  });

  // --- Authenticated endpoints (skipped without a token) -------------------
  if (!token) {
    if (auth.source === 'login-failed') {
      check(null, { 'auth login succeeded': () => false });
      console.log(`!! login failed (status ${auth.status}) — skipping authenticated checks`);
    } else {
      console.log('i no TEST_TOKEN/TEST_EMAIL+TEST_PASSWORD — skipping authenticated checks');
    }
    return;
  }

  group('authenticated', () => {
    const profile = apiGet('/profile/me', token, { endpoint: 'profile_me' });
    check(profile, {
      'GET /profile/me 200': (r) => r.status === 200,
      'GET /profile/me returns user id': (r) => {
        const b = safeJson(r);
        return b && b.data && !!b.data.id;
      },
    });

    const habits = apiGet('/habits?page=1&limit=10', token, { endpoint: 'habits_list' });
    check(habits, {
      'GET /habits 200': (r) => r.status === 200,
      'GET /habits returns data[]': (r) => {
        const b = safeJson(r);
        return b && Array.isArray(b.data);
      },
    });

    const goals = apiGet('/goals?page=1&limit=10', token, { endpoint: 'goals_list' });
    check(goals, {
      'GET /goals 200': (r) => r.status === 200,
      'GET /goals returns data[]': (r) => {
        const b = safeJson(r);
        return b && Array.isArray(b.data);
      },
    });

    const today = apiGet('/check-ins/today', token, { endpoint: 'checkins_today' });
    check(today, {
      'GET /check-ins/today 200': (r) => r.status === 200,
      'GET /check-ins/today returns checkIns[]': (r) => {
        const b = safeJson(r);
        return b && Array.isArray(b.checkIns);
      },
    });

    const unread = apiGet('/notifications/unread-count', token, { endpoint: 'unread_count' });
    check(unread, {
      'GET /notifications/unread-count 200': (r) => r.status === 200,
      'unread-count returns count': (r) => {
        const b = safeJson(r);
        return b && typeof b.count === 'number';
      },
    });

    const userSettings = apiGet('/settings', token, { endpoint: 'settings_get' });
    check(userSettings, { 'GET /settings 200': (r) => r.status === 200 });

    const activity = apiGet('/activity?page=1&limit=10', token, { endpoint: 'activity' });
    check(activity, { 'GET /activity 200': (r) => r.status === 200 });

    // ai-gateway read routes (conversations + memory live behind ai-gateway).
    const convos = apiGet('/conversations?page=1&limit=10', token, { endpoint: 'conversations' });
    check(convos, { 'GET /conversations 200': (r) => r.status === 200 });

    const facts = apiGet('/memory/facts?page=1&limit=10', token, { endpoint: 'memory_facts' });
    check(facts, { 'GET /memory/facts 200': (r) => r.status === 200 });

    // May 404 if the user has no weekly review yet — both are valid.
    const currentReview = apiGet('/weekly-reviews/current', token, {
      endpoint: 'weekly_review_current',
      expected: EXPECTED_READ_OR_404,
    });
    check(currentReview, {
      'GET /weekly-reviews/current 200 or 404': (r) => r.status === 200 || r.status === 404,
    });
  });

  sleep(0.3);
}
