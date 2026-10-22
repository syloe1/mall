// P2 管理端 — 小并发抽样
// 场景: 管理员查询商品/死信列表
// 运行: k6 run k6/scenarios/p2_admin.js

import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE = 'http://localhost:3333/api/v1';

export const options = {
  vus: 5,
  duration: '30s',
  thresholds: {
    'http_req_duration': ['p(99)<500'],
    'http_req_failed': ['rate<0.01'],
  },
};

export function setup() {
  // 需提前在 DB 中设置一个 admin 角色用户
  const login = http.post(`${BASE}/users/login`, JSON.stringify({
    username: 'admin', password: 'admin123',
  }), { headers: { 'Content-Type': 'application/json' } });
  const token = login.json().data?.access_token || '';
  return { token };
}

export default function (data) {
  if (!data.token) return;
  const auth = { 'Content-Type': 'application/json', Authorization: `Bearer ${data.token}` };

  http.get(`${BASE}/admin/products?page=1&page_size=20`, { headers: auth });
  http.get(`${BASE}/admin/dead-letters/products?page=1&page_size=20`, { headers: auth });

  sleep(5);
}
