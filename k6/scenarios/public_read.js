// 公开只读接口压测（/about 页面「压测数据」一节的数据来源）
//
// 用法：
//   k6 run -e SCENARIO=products -e BASE=https://mall.syloe1.xyz/api/v1 k6/scenarios/public_read.js
//   k6 run -e SCENARIO=search   ...
//   k6 run -e SCENARIO=categories ...
//   k6 run -e SCENARIO=mixed    ...
//
// 设计：100 VU 爬坡 15s → 稳态 30s → 收尾 5s；只读、不写业务数据。
import http from 'k6/http'
import { check } from 'k6'

const BASE = __ENV.BASE || 'http://localhost:18888/api/v1'
const SCENARIO = __ENV.SCENARIO || 'products'

export const options = {
  scenarios: {
    [SCENARIO]: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '15s', target: 100 },
        { duration: '30s', target: 100 },
        { duration: '5s', target: 0 },
      ],
      gracefulRampDown: '5s',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.02'],
    http_req_duration: ['p(95)<1500'],
  },
}

const PATHS = {
  products: () => `${BASE}/products?page=1&page_size=12`,
  search: () => `${BASE}/products/search?keyword=${encodeURIComponent('耳机')}&page=1&page_size=12`,
  categories: () => `${BASE}/categories`,
  detail: () => `${BASE}/products/1001`,
  ping: () => `${BASE}/ping`,
}

export default function () {
  let url
  switch (SCENARIO) {
    case 'search':
      url = PATHS.search()
      break
    case 'categories':
      url = PATHS.categories()
      break
    case 'mixed': {
      const urls = [PATHS.products(), PATHS.detail(), PATHS.categories(), PATHS.search()]
      url = urls[Math.floor(Math.random() * urls.length)]
      break
    }
    default:
      url = PATHS.products()
  }
  const res = http.get(url, { tags: { scenario: SCENARIO } })
  check(res, { 'status is 200': (r) => r.status === 200 })
}
