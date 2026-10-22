// P1 用户侧接口 — 梯度加压
import http from 'k6/http';
import { sleep } from 'k6';

const BASE = 'http://localhost:3333/api/v1';

export const options = {
  stages: [
    { duration: '10s', target: 100 },
    { duration: '20s', target: 400 },
    { duration: '20s', target: 0 },
  ],
  thresholds: {
    'http_req_duration': ['p(99)<250'],
    'http_req_failed': ['rate<0.005'],
  },
};

export function setup() {
  const tokens = [];
  for (let i = 0; i < 10; i++) {
    const body = JSON.stringify({ username: `perf_user_${i}`, password: '123456' });
    http.post(`${BASE}/users/register`, body, { headers: { 'Content-Type': 'application/json' } });
    const login = http.post(`${BASE}/users/login`, body, { headers: { 'Content-Type': 'application/json' } });
    const t = login.json().data?.access_token;
    if (t) tokens.push(t);
  }
  return JSON.stringify(tokens);
}

export default function (data) {
  const tokens = JSON.parse(data);
  if (!tokens.length) return;
  const token = tokens[Math.floor(Math.random() * tokens.length)];
  const auth = { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` };

  http.get(`${BASE}/ping`);
  http.get(`${BASE}/users/me/amount`, { headers: auth });
  http.get(`${BASE}/users/addresses`, { headers: auth });
  http.get(`${BASE}/users/cart/items`, { headers: auth });

  sleep(Math.random() * 1.5 + 0.5);
}
