// Mall API 极限压测 — 找出瓶颈
// 运行: k6 run k6/stress_test.js

import http from 'k6/http';
import { check } from 'k6';

const BASE = 'http://localhost:3333/api/v1';

export const options = {
  stages: [
    { duration: '15s', target: 100 },
    { duration: '15s', target: 300 },
    { duration: '15s', target: 500 },
    { duration: '20s', target: 800 },
    { duration: '15s', target: 0 },
  ],
  thresholds: {
    'http_req_duration': ['p(95)<2000'],
    'http_req_failed': ['rate<0.05'],
  },
};

export default function () {
  // 无 sleep，纯压测
  http.get(`${BASE}/ping`);
  http.get(`${BASE}/products?page=1&page_size=20`);
  http.get(`${BASE}/products?page=2&page_size=20`);
}
