// load.js — realistic mixed-traffic load test for the Growth API.
//
// Ramps 0 → ~50% → MAX_VUS concurrent "users" over ~10 minutes. Each VU
// performs one weighted user action per iteration (mostly reads, a small
// share of self-cleaning writes when credentials are available) with 1–4s
// of think time between actions.
//
//   k6 run load.js                                  # public traffic only
//   TEST_TOKEN=<jwt> k6 run load.js                 # + authenticated mix
//   TEST_EMAIL=.. TEST_PASSWORD=.. k6 run load.js   # token via one login
//   MAX_VUS=100 k6 run load.js                      # override peak VUs
//
// NOTE: a single shared token means all authed traffic is one user. Auth
// endpoints are NOT exercised here — login happens once in setup() because
// /api/v1/auth/* is rate-limited (~10 req/min per IP).
//
import { check, sleep } from 'k6';
import {
  EXPECTED_READ_OR_404,
  EXPECTED_WRITE,
  acquireToken,
  apiDel,
  apiGet,
  apiPost,
  apiPut,
  pick,
  pickWeighted,
  randomIntBetween,
  safeJson,
} from './lib.js';

const MAX_VUS = parseInt(__ENV.MAX_VUS || '50', 10);
const HALF = Math.max(1, Math.floor(MAX_VUS / 2));

export const options = {
  scenarios: {
    mixed_traffic: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '2m', target: HALF }, // ramp up to half
        { duration: '3m', target: HALF }, // hold
        { duration: '2m', target: MAX_VUS }, // ramp to peak
        { duration: '2m', target: MAX_VUS }, // hold at peak
        { duration: '1m', target: 0 }, // ramp down
      ],
      gracefulRampDown: '30s',
    },
  },
  thresholds: {
    // Baseline capacity target: reads stay under 500ms at p95.
    http_req_failed: ['rate<0.01'],
    'http_req_duration{kind:read}': ['p(95)<500', 'p(99)<1500'],
    'http_req_duration{kind:write}': ['p(95)<1500'],
    checks: ['rate>0.95'],
  },
};

export function setup() {
  const auth = acquireToken();
  if (auth.source === 'login-failed') {
    console.log(`!! login failed (status ${auth.status}) — continuing with public traffic only`);
  } else if (auth.source === 'none') {
    console.log('i no credentials provided — public traffic only');
  }
  return auth;
}

// ---------------------------------------------------------------------------
// Actions. Each is a small user flow: 1–3 related requests.
// ---------------------------------------------------------------------------

const SEARCH_TERMS = ['habit', 'goal', 'focus', 'morning', 'health', 'motivation'];

// -- public reads (always in the mix) --

function listArticles() {
  const res = apiGet('/articles?page=1&limit=10', '', { endpoint: 'articles_list' });
  check(res, {
    'articles list ok': (r) => r.status === 200,
    'articles list has data': (r) => {
      const b = safeJson(r);
      return b && Array.isArray(b.data);
    },
  });
}

// List → open one article, like a real reader.
function readArticle() {
  const list = apiGet('/articles?page=1&limit=10', '', { endpoint: 'articles_list' });
  const body = safeJson(list);
  if (!body || !Array.isArray(body.data) || body.data.length === 0) return;
  const article = pick(body.data);
  const res = apiGet(`/articles/${article.id}`, '', { endpoint: 'article_detail' });
  check(res, { 'article detail ok': (r) => r.status === 200 });
}

function listCategories() {
  const res = apiGet(`/categories?entityType=${pick(['habit', 'goal', 'article'])}`, '', {
    endpoint: 'categories',
  });
  check(res, { 'categories ok': (r) => r.status === 200 });
}

function siteSettings() {
  const res = apiGet('/site-settings', '', { endpoint: 'site_settings' });
  check(res, { 'site-settings ok': (r) => r.status === 200 });
}

function habitTemplates() {
  const res = apiGet('/habit-templates', '', { endpoint: 'habit_templates' });
  check(res, { 'habit-templates ok': (r) => r.status === 200 });
}

function goalTemplates() {
  const res = apiGet('/goal-templates', '', { endpoint: 'goal_templates' });
  check(res, { 'goal-templates ok': (r) => r.status === 200 });
}

// Public search is per-IP rate-limited (~30/min) — keep its weight small.
function search() {
  const res = apiGet(`/search?q=${pick(SEARCH_TERMS)}&page=1&limit=10`, '', { endpoint: 'search' });
  check(res, { 'search ok': (r) => r.status === 200 });
}

// -- authenticated reads --

function profile(token) {
  const res = apiGet('/profile/me', token, { endpoint: 'profile_me' });
  check(res, { 'profile ok': (r) => r.status === 200 });
}

function listHabits(token) {
  const res = apiGet('/habits?page=1&limit=20', token, { endpoint: 'habits_list' });
  check(res, { 'habits ok': (r) => r.status === 200 });
}

function listGoals(token) {
  const res = apiGet('/goals?page=1&limit=20', token, { endpoint: 'goals_list' });
  check(res, { 'goals ok': (r) => r.status === 200 });
}

function todayCheckIns(token) {
  const res = apiGet('/check-ins/today', token, { endpoint: 'checkins_today' });
  check(res, { 'check-ins today ok': (r) => r.status === 200 });
}

function notifications(token) {
  const res = apiGet('/notifications?page=1&limit=20', token, { endpoint: 'notifications' });
  check(res, { 'notifications ok': (r) => r.status === 200 });
}

function unreadCount(token) {
  const res = apiGet('/notifications/unread-count', token, { endpoint: 'unread_count' });
  check(res, { 'unread-count ok': (r) => r.status === 200 });
}

function getSettings(token) {
  const res = apiGet('/settings', token, { endpoint: 'settings_get' });
  check(res, { 'settings ok': (r) => r.status === 200 });
}

function activityFeed(token) {
  const res = apiGet('/activity?page=1&limit=20', token, { endpoint: 'activity' });
  check(res, { 'activity ok': (r) => r.status === 200 });
}

function savedItems(token) {
  const res = apiGet('/saved?page=1&limit=20', token, { endpoint: 'saved' });
  check(res, { 'saved ok': (r) => r.status === 200 });
}

function weeklyReviews(token) {
  const res = apiGet('/weekly-reviews?page=1&limit=10', token, { endpoint: 'weekly_reviews' });
  check(res, { 'weekly-reviews ok': (r) => r.status === 200 });
}

function currentWeeklyReview(token) {
  const res = apiGet('/weekly-reviews/current', token, {
    endpoint: 'weekly_review_current',
    expected: EXPECTED_READ_OR_404,
  });
  check(res, { 'current weekly review ok': (r) => r.status === 200 || r.status === 404 });
}

function billingOverview(token) {
  const res = apiGet('/billing/overview', token, { endpoint: 'billing_overview' });
  check(res, { 'billing overview ok': (r) => r.status === 200 });
}

function coachingProfile(token) {
  const res = apiGet('/personalization/coaching-profile', token, { endpoint: 'coaching_profile' });
  check(res, { 'coaching profile ok': (r) => r.status === 200 });
}

// ai-gateway read routes (conversations + memory are served by :8889).
function conversations(token) {
  const res = apiGet('/conversations?page=1&limit=20', token, { endpoint: 'conversations' });
  check(res, { 'conversations ok': (r) => r.status === 200 });
}

function memoryFacts(token) {
  const res = apiGet('/memory/facts?page=1&limit=20', token, { endpoint: 'memory_facts' });
  check(res, { 'memory facts ok': (r) => r.status === 200 });
}

// -- writes (self-cleaning, low weight) --

// Idempotent-ish settings write against the dedicated test user. Overwrites
// theme/timezone/checkInTime — run against a test account, not a real user.
function updateSettings(token) {
  const res = apiPut(
    '/settings',
    { theme: 'dark', timezone: 'UTC', checkInTime: '09:00' },
    token,
    { endpoint: 'settings_put', expected: EXPECTED_WRITE }
  );
  check(res, { 'settings update ok': (r) => r.status >= 200 && r.status < 300 });
}

// Create a habit then immediately delete it — exercises the write path while
// leaving the test account clean.
function createAndDeleteHabit(token) {
  const name = `k6-loadtest-vu${__VU}-it${__ITER}-${Date.now()}`;
  const created = apiPost(
    '/habits',
    { name, description: 'ephemeral k6 load-test habit', category: 'fitness' },
    token,
    { endpoint: 'habits_create', expected: EXPECTED_WRITE }
  );
  check(created, { 'habit create ok': (r) => r.status >= 200 && r.status < 300 });
  const body = safeJson(created);
  if (body && body.data && body.data.id) {
    const del = apiDel(`/habits/${body.data.id}`, token, {
      endpoint: 'habits_delete',
      expected: EXPECTED_WRITE,
    });
    check(del, { 'habit delete ok': (r) => r.status >= 200 && r.status < 300 });
  }
}

// Check-in flow: pick a habit, skip if already checked in today, else POST.
// NOTE: POST /check-ins is in the AI rate-limit bucket (~10/min/user) and
// enqueues async AI feedback — keep its weight small.
function checkInFlow(token) {
  const habits = apiGet('/habits?page=1&limit=20', token, { endpoint: 'habits_list' });
  const body = safeJson(habits);
  if (!body || !Array.isArray(body.data) || body.data.length === 0) return;
  const habit = pick(body.data);

  const checked = apiGet(`/check-ins/checked-today?habitId=${habit.id}`, token, {
    endpoint: 'checkins_checked_today',
  });
  const c = safeJson(checked);
  if (!c || c.checkedIn !== false) return; // nothing unchecked → skip write

  const res = apiPost(
    '/check-ins',
    { habitId: habit.id, status: 'completed', mood: 'great', energy: 'high', note: 'k6 load test' },
    token,
    { endpoint: 'checkins_create', expected: EXPECTED_WRITE }
  );
  check(res, {
    'check-in ok': (r) => r.status >= 200 && r.status < 300,
  });
}

// ---------------------------------------------------------------------------
// Weighted action table. Weights are relative. Public reads dominate; authed
// reads and writes only exist when a token is available.
// ---------------------------------------------------------------------------

function buildActions(authed) {
  const actions = [
    { weight: 20, name: 'list_articles', run: () => listArticles() },
    { weight: 12, name: 'read_article', run: () => readArticle() },
    { weight: 5, name: 'categories', run: () => listCategories() },
    { weight: 4, name: 'site_settings', run: () => siteSettings() },
    { weight: 5, name: 'habit_templates', run: () => habitTemplates() },
    { weight: 4, name: 'goal_templates', run: () => goalTemplates() },
    { weight: 3, name: 'search', run: () => search() },
  ];
  if (authed) {
    actions.push(
      { weight: 5, name: 'profile', run: (t) => profile(t) },
      { weight: 7, name: 'habits', run: (t) => listHabits(t) },
      { weight: 5, name: 'goals', run: (t) => listGoals(t) },
      { weight: 4, name: 'checkins_today', run: (t) => todayCheckIns(t) },
      { weight: 4, name: 'notifications', run: (t) => notifications(t) },
      { weight: 3, name: 'unread_count', run: (t) => unreadCount(t) },
      { weight: 4, name: 'settings_get', run: (t) => getSettings(t) },
      { weight: 3, name: 'activity', run: (t) => activityFeed(t) },
      { weight: 3, name: 'saved', run: (t) => savedItems(t) },
      { weight: 3, name: 'weekly_reviews', run: (t) => weeklyReviews(t) },
      { weight: 3, name: 'weekly_review_current', run: (t) => currentWeeklyReview(t) },
      { weight: 2, name: 'billing_overview', run: (t) => billingOverview(t) },
      { weight: 2, name: 'coaching_profile', run: (t) => coachingProfile(t) },
      { weight: 3, name: 'conversations', run: (t) => conversations(t) },
      { weight: 2, name: 'memory_facts', run: (t) => memoryFacts(t) },
      // writes — self-cleaning, small share (~6%)
      { weight: 2, name: 'settings_put', run: (t) => updateSettings(t) },
      { weight: 2, name: 'habit_create_delete', run: (t) => createAndDeleteHabit(t) },
      { weight: 2, name: 'checkin', run: (t) => checkInFlow(t) }
    );
  }
  return actions;
}

let actions = null;

export default function (auth) {
  if (actions === null) {
    actions = buildActions(!!auth.token);
  }
  const action = pickWeighted(actions);
  action.run(auth.token);
  sleep(randomIntBetween(1, 4));
}
