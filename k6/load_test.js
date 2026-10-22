// Mall API 压力测试
// 运行: k6 run k6/load_test.js

import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE = 'http://localhost:3333/api/v1';

export const options = {
  stages: [
    { duration: '10s', target: 10 },
    { duration: '15s', target: 50 },
    { duration: '20s', target: 100 },
    { duration: '15s', target: 200 },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    'http_req_duration': ['p(95)<1000'],
    'http_req_failed': ['rate<0.01'],
  },
};

export function setup() {
  const payload = JSON.stringify({ username: 'loaduser', password: '123456' });
  http.post(`${BASE}/users/register`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });
  const loginRes = http.post(`${BASE}/users/login`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });
  const body = loginRes.json();
  const token = body.data ? body.data.access_token : '';
  console.log(`Token: ${token ? 'OK' : 'FAIL'}`);
  return { token };
}

export default function (data) {
  const json = { 'Content-Type': 'application/json' };
  const auth = { ...json, Authorization: `Bearer ${data.token}` };

  // 1. 健康检查（最轻）
  http.get(`${BASE}/ping`);

  // 2. 商品列表（公开，查 MySQL）
  const p = http.get(`${BASE}/products?page=1&page_size=10`);
  check(p, { 'prod 200': (r) => r.status === 200 });

  // 3. 认证接口（有 token 时）
  if (data.token) {
    http.get(`${BASE}/users/cart/items`, { headers: auth });
    http.get(`${BASE}/users/addresses`, { headers: auth });
  }

  sleep(Math.random() * 1.5 + 0.5);
}

export function teardown() {
  console.log('Done.');
}
