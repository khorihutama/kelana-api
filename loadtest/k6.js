import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const BASE = __ENV.BASE_URL || 'http://127.0.0.1:8080';
const EMAIL = __ENV.LOADTEST_EMAIL || '';
const PASSWORD = __ENV.LOADTEST_PASSWORD || '';
const DURATION = __ENV.LOADTEST_DURATION || '30s';

const businessErrors = new Rate('business_errors');

const scenarios = {
  discovery: {
    executor: 'constant-vus',
    vus: Number(__ENV.LOADTEST_VUS || 20),
    duration: DURATION,
    exec: 'discovery',
  },
};

if (EMAIL && PASSWORD) {
  scenarios.login = {
    executor: 'per-vu-iterations',
    vus: 1,
    iterations: 3,
    startTime: '2s',
    exec: 'loginFlow',
  };
  scenarios.booking = {
    executor: 'per-vu-iterations',
    vus: 1,
    iterations: 5,
    startTime: '5s',
    exec: 'bookingFlow',
  };
}

export const options = {
  scenarios,
  thresholds: {
    'http_req_duration{scenario:discovery}': ['p(95)<800'],
    http_req_failed: ['rate<0.02'],
    business_errors: ['rate<0.02'],
  },
  summaryTrendStats: ['avg', 'min', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

function jsonHeaders(token) {
  const headers = { 'Content-Type': 'application/json' };
  if (token) headers.Authorization = `Bearer ${token}`;
  return headers;
}

export function setup() {
  if (!EMAIL || !PASSWORD) return {};
  const login = http.post(`${BASE}/api/v1/auth/login`, JSON.stringify({ email: EMAIL, password: PASSWORD }), {
    headers: jsonHeaders(),
    tags: { name: 'setup_login' },
  });
  if (login.status !== 200) return { setupError: `login ${login.status}` };
  const token = login.json('data.access_token');

  const trips = http.get(`${BASE}/api/v1/trips?limit=10`, { tags: { name: 'setup_trips' } });
  if (trips.status !== 200) return { token, setupError: `trips ${trips.status}` };
  const tripID = trips.json('data.0.id');
  const detail = http.get(`${BASE}/api/v1/trips/${tripID}`, { tags: { name: 'setup_trip_detail' } });
  if (detail.status !== 200) return { token, setupError: `detail ${detail.status}` };
  const schedules = detail.json('data.schedules') || [];
  const schedule = schedules.find((item) => item.status === 'OPEN' && item.seats_available > 0);
  const hotels = detail.json('data.hotel_options') || [];
  return {
    token,
    scheduleID: schedule ? schedule.id : '',
    hotelID: hotels.length > 0 ? hotels[0].id : '',
  };
}

export function discovery() {
  const page = (__ITER % 3) + 1;
  const list = http.get(`${BASE}/api/v1/trips?limit=10&page=${page}`, {
    tags: { name: 'GET /trips' },
  });
  check(list, { 'trips 200': (r) => r.status === 200 });
  businessErrors.add(list.status !== 200);

  const tripID = list.json('data.0.id');
  if (!tripID) return;
  const detail = http.get(`${BASE}/api/v1/trips/${tripID}`, {
    tags: { name: 'GET /trips/{id}' },
  });
  check(detail, { 'detail 200': (r) => r.status === 200 });
  businessErrors.add(detail.status !== 200);

  if (__ITER % 2 === 0) {
    const destinations = http.get(`${BASE}/api/v1/destinations`, {
      tags: { name: 'GET /destinations' },
    });
    check(destinations, { 'destinations 200': (r) => r.status === 200 });
    businessErrors.add(destinations.status !== 200);
  } else {
    const categories = http.get(`${BASE}/api/v1/categories`, {
      tags: { name: 'GET /categories' },
    });
    check(categories, { 'categories 200': (r) => r.status === 200 });
    businessErrors.add(categories.status !== 200);
  }
  sleep(0.1);
}

export function loginFlow() {
  const login = http.post(`${BASE}/api/v1/auth/login`, JSON.stringify({ email: EMAIL, password: PASSWORD }), {
    headers: jsonHeaders(),
    tags: { name: 'POST /auth/login' },
  });
  check(login, { 'login 200': (r) => r.status === 200 });
  businessErrors.add(login.status !== 200);
  const token = login.json('data.access_token');
  if (token) {
    const me = http.get(`${BASE}/api/v1/auth/me`, {
      headers: jsonHeaders(token),
      tags: { name: 'GET /auth/me' },
    });
    check(me, { 'me 200': (r) => r.status === 200 });
    businessErrors.add(me.status !== 200);
  }
}

export function bookingFlow(data) {
  if (!data || !data.token || !data.scheduleID) return;
  const payload = {
    schedule_id: data.scheduleID,
    participants: [{
      full_name: 'Load Test',
      gender: 'MALE',
      phone: '081200000099',
      email: 'loadtest-participant@example.com',
    }],
    pickup_point: 'Jakarta Pusat',
  };
  if (data.hotelID) payload.hotel_id = data.hotelID;

  const lock = http.post(`${BASE}/api/v1/reservations/lock`, JSON.stringify(payload), {
    headers: jsonHeaders(data.token),
    tags: { name: 'POST /reservations/lock' },
  });
  const locked = lock.status === 201;
  check(lock, { 'lock 201': () => locked });
  businessErrors.add(lock.status !== 201 && lock.status !== 409);

  if (!locked) return;
  const reservationID = lock.json('data.id');
  const cancel = http.post(`${BASE}/api/v1/reservations/${reservationID}/cancel`, null, {
    headers: jsonHeaders(data.token),
    tags: { name: 'POST /reservations/{id}/cancel' },
  });
  check(cancel, { 'cancel 200': (r) => r.status === 200 });
  businessErrors.add(cancel.status !== 200);
  sleep(0.3);
}