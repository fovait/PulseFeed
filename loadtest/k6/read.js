// 读路径压测：实验 A(基线爬坡) + 实验 B(缓存命中 vs 冷兜底)。
// 这些接口无鉴权、无限流，是最干净的压测目标。
//
//   k6 run read.js                       # 实验A: listLatest 首页爬坡(缓存友好)
//   TARGET=detail MAXID=2000 k6 run read.js
//   SKEW=2592000 k6 run read.js          # 实验B: 随机 before_time 打散缓存,逼出冷路径
//   TARGET=popularity k6 run read.js
//
// 环境变量:
//   BASE   目标地址          (默认 http://localhost:8080)
//   TARGET latest|popularity|detail (默认 latest)
//   SKEW   >0 时 listLatest 用随机 before_time(过去 SKEW 秒内),制造缓存 miss
//   MINID/MAXID  detail 随机取的视频 id 区间(由 seeder 输出)
import http from 'k6/http';
import { check } from 'k6';

const BASE = __ENV.BASE || 'http://localhost:8080';
const TARGET = __ENV.TARGET || 'latest';
const SKEW = Number(__ENV.SKEW || 0);
const MINID = Number(__ENV.MINID || 1);
const MAXID = Number(__ENV.MAXID || 2000);

export const options = {
  stages: [
    { duration: '30s', target: 50 },
    { duration: '1m', target: 150 },
    { duration: '1m', target: 300 },
    { duration: '1m', target: 500 },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<300', 'p(99)<800'],
  },
};

const H = { headers: { 'Content-Type': 'application/json' } };

function randID() {
  return Math.floor(Math.random() * (MAXID - MINID + 1)) + MINID;
}

function fire() {
  if (TARGET === 'detail') {
    return http.post(`${BASE}/video/getDetail`, JSON.stringify({ id: randID() }), H);
  }
  if (TARGET === 'popularity') {
    return http.post(`${BASE}/feed/listByPopularity`, JSON.stringify({ limit: 10 }), H);
  }
  // latest(默认)
  const body = { limit: 10 };
  if (SKEW > 0) {
    body.before_time = Math.floor(Date.now() / 1000) - Math.floor(Math.random() * SKEW);
  }
  return http.post(`${BASE}/feed/listLatest`, JSON.stringify(body), H);
}

export default function () {
  const res = fire();
  check(res, { 'status 200': (r) => r.status === 200 });
}
