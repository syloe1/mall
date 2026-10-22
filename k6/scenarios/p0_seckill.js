// P0 秒杀 — 瞬时并发冲击
// 场景: 1500 VU 瞬时并发抢 100 件库存
// 运行: k6 run k6/scenarios/p0_seckill.js

import http from 'k6/http';
import { check } from 'k6';

const BASE = 'http://localhost:3333/api/v1';
const ACTIVITY_ID = __ENV.ACTIVITY_ID || '1';

export const options = {
  scenarios: {
    spike: {
      executor: 'ramping-arrival-rate',
      startRate: 100,
      timeUnit: '1s',
      preAllocatedVUs: 500,
      maxVUs: 2000,
      stages: [
        { duration: '5s',  target: 500 },
        { duration: '5s',  target: 1500 },  // 瞬间 1500 req/s
        { duration: '10s', target: 1500 },  // 维持
        { duration: '5s',  target: 0 },
      ],
    },
  },
  thresholds: {
    'http_req_duration': ['p(99)<500'],
    'http_req_failed': ['rate<0.95'],  // 大部分会失败（售罄/已买/限流），允许高失败率
  },
};

export function setup() {
  // 批量获取 token
  const tokens = [];
  for (let i = 1; i <= 50; i++) {
    const u = { username: `seckiller_${i}`, password: '123456' };
    const body = JSON.stringify(u);
    http.post(`${BASE}/users/register`, body, { headers: { 'Content-Type': 'application/json' } });
    const login = http.post(`${BASE}/users/login`, body, { headers: { 'Content-Type': 'application/json' } });
    const t = login.json().data?.access_token;
    if (t) tokens.push(t);
  }
  return { tokens };
}

export default function (data) {
  const token = data.tokens[Math.floor(Math.random() * data.tokens.length)];
  if (!token) return;

  const res = http.post(`${BASE}/seckill/activities/${ACTIVITY_ID}/buy`, null, {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
  });

  check(res, {
    'status ok': (r) => [200, 400, 429].includes(r.status),
  });
}
