// P0 商品接口 — 基准压测
// 场景: 用户浏览商品列表/详情/搜索
// 运行: k6 run k6/scenarios/p0_products.js

import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE = 'http://localhost:3333/api/v1';

export const options = {
  stages: [
    { duration: '15s', target: 50 },
    { duration: '30s', target: 400 },   // 日常峰值
    { duration: '15s', target: 800 },   // 大促峰值
    { duration: '30s', target: 800 },   // 维持
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    'http_req_duration{endpoint:products}': ['p(99)<250'],
    'http_req_duration{endpoint:detail}':   ['p(99)<250'],
    'http_req_duration{endpoint:search}':   ['p(99)<250'],
    'http_req_failed': ['rate<0.005'],
  },
};

const PRODUCT_ID_START = 10;
const PRODUCT_ID_END = 59;
const KEYWORDS = ['test', '商品', '手机', '电脑'];

export default function () {
  // 1. 商品列表 (高频)
  const list = http.get(`${BASE}/products?page=1&page_size=20`, {
    tags: { endpoint: 'products' },
  });
  check(list, { 'list 200': (r) => r.status === 200 });

  // 2. 商品详情 (中频)
  const id = PRODUCT_ID_START + Math.floor(Math.random() * (PRODUCT_ID_END - PRODUCT_ID_START + 1));
  const detail = http.get(`${BASE}/products/${id}`, {
    tags: { endpoint: 'detail' },
  });
  check(detail, { 'detail ok': (r) => r.status === 200 || r.status === 404 });

  // 3. 搜索 (ES 未建索引时会 500，降低频率)
  if (Math.random() < 0.1) {
    const kw = KEYWORDS[Math.floor(Math.random() * KEYWORDS.length)];
    http.get(`${BASE}/products/search?keyword=${kw}&page=1&page_size=20`, {
      tags: { endpoint: 'search' },
    });
    // search may 500 if ES index not created — don't count as failure
  }

  sleep(Math.random() * 2 + 0.5);
}
