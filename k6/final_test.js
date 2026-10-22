import http from 'k6/http';

const B = 'http://localhost:3333/api/v1';
const PIDS = Array.from({length: 50}, (_, i) => 10 + i); // IDs 10-59

export const options = {
  stages: [
    { duration: '10s', target: 10 },
    { duration: '20s', target: 200 },
    { duration: '30s', target: 500 },
    { duration: '10s', target: 0 },
  ],
};

export default function () {
  // 公开接口（无需登录）
  http.get(`${B}/ping`);
  http.get(`${B}/products?page=1&page_size=20`);
  http.get(`${B}/products/${PIDS[Math.floor(Math.random() * 50)]}`);
}
