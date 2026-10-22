// P0 下单+支付 — 梯度加压
import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE = 'http://localhost:3333/api/v1';

export const options = {
  stages: [
    { duration: '10s', target: 10 },
    { duration: '15s', target: 100 },
    { duration: '15s', target: 200 },
    { duration: '20s', target: 0 },
  ],
  thresholds: {
    'http_req_duration': ['p(99)<500'],
    'http_req_failed': ['rate<0.01'],
  },
};

let tokens = [];
let addrIDs = [];

export function setup() {
  const users = [];
  for (let i = 0; i < 20; i++) {
    const username = `perf_order_${i}`;
    const body = JSON.stringify({ username, password: '123456' });
    const reg = http.post(`${BASE}/users/register`, body, { headers: { 'Content-Type': 'application/json' } });
    const login = http.post(`${BASE}/users/login`, body, { headers: { 'Content-Type': 'application/json' } });
    const t = login.json().data?.access_token;
    if (!t) continue;
    const auth = { 'Content-Type': 'application/json', Authorization: `Bearer ${t}` };
    const addrRes = http.post(`${BASE}/users/addresses`, JSON.stringify({
      receiver_name: 'Test', phone: '13800138000',
      province: '北京', city: '北京', district: '朝阳', detail: 'test',
    }), { headers: auth });
    const addrID = addrRes.json().data?.ID || 1;
    users.push({ token: t, addressID: addrID });
  }
  return JSON.stringify(users);
}

export default function (data) {
  const users = JSON.parse(data);
  if (!users.length) return;
  const u = users[Math.floor(Math.random() * users.length)];
  const auth = { 'Content-Type': 'application/json', Authorization: `Bearer ${u.token}` };

  http.post(`${BASE}/users/cart/items`, JSON.stringify({
    product_id: 10 + Math.floor(Math.random() * 50),
    quantity: 1,
  }), { headers: auth });

  const order = http.post(`${BASE}/users/orders/cart`, JSON.stringify({
    address_id: u.addrID,
  }), { headers: auth });

  if (order.status === 200 && order.json().data?.ID) {
    http.post(`${BASE}/users/payments`, JSON.stringify({
      order_id: order.json().data.ID,
      channel: 'mock',
    }), { headers: auth });
  }

  sleep(Math.random() * 2 + 1);
}
