// 写路径压测：实验 C(同步入队 vs 异步落库)。
// 多账号轮换:每个 VU 用不同的 bench 账号点赞随机视频,避免"单账号点满固定视频池"
// 导致后期全是重复点赞(409)的假象,从而测出真实的并发写吞吐。
//
// 量的是【HTTP 入队延迟】应保持平稳(异步落库);真正的吞吐看 RabbitMQ 队列积压与
// 排空速度(见 run.sh watch-mq)。
//
// 前置:API 必须以 RATELIMIT_DISABLED=1 启动(否则 login/like 限流会把压测限死);
//       数据由 seeder 灌好(默认 100 账号 bench_0001..bench_0100 / 密码 bench123)。
//
//   MINID=1 MAXID=2007 k6 run write.js
//
// 环境变量:
//   BASE          目标地址            (默认 http://localhost:8080)
//   BENCH_PREFIX  账号前缀            (默认 bench,需与 seeder -prefix 一致)
//   BENCH_USERS   轮换的账号数        (默认 100,需 ≤ seeder 实际创建数)
//   BENCH_PASS    账号密码            (默认 bench123)
//     ⚠️ 不要用 USER:它是 Unix 标准环境变量(=$USER),k6 的 __ENV 会读到它而非默认值
//   MINID/MAXID   随机点赞的视频 id 区间(由 seeder 输出)
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter } from 'k6/metrics';

// 把 409(重复点赞这类业务冲突)视为"预期响应",不计入 http_req_failed。
// 这样错误率只反映真正的服务器故障(5xx)。
http.setResponseCallback(http.expectedStatuses(200, 409));

const BASE = __ENV.BASE || 'http://localhost:8080';
const PREFIX = __ENV.BENCH_PREFIX || 'bench';
const PASS = __ENV.BENCH_PASS || 'bench123';
const NUM_USERS = Number(__ENV.BENCH_USERS || 100);
const MINID = Number(__ENV.MINID || 1);
const MAXID = Number(__ENV.MAXID || 2000);

const likesNew = new Counter('likes_new'); // 200:真正新点赞,已入队
const likesDup = new Counter('likes_dup'); // 409:重复点赞(幂等冲突,正常)

export const options = {
  stages: [
    { duration: '30s', target: 50 },
    { duration: '1m', target: 200 },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    http_req_failed: ['rate<0.02'], // 只剩真 5xx 会被计入
    http_req_duration: ['p(99)<500'], // 异步入队应当很快
  },
};

// 账号名与 seeder 的 fmt.Sprintf("%s_%04d", prefix, i) 对齐:bench_0001 ...
function username(i) {
  return `${PREFIX}_${String(i).padStart(4, '0')}`;
}

// 登录带重试:紧贴重负载实验后跑时服务器可能还在饱和,首次登录偶发失败。
function login(user) {
  for (let attempt = 1; attempt <= 5; attempt++) {
    const r = http.post(`${BASE}/account/login`, JSON.stringify({ username: user, password: PASS }), {
      headers: { 'Content-Type': 'application/json' },
    });
    if (r.status === 200) {
      const token = r.json('token');
      if (token) {
        return token;
      }
    }
    sleep(1);
  }
  return null;
}

export function setup() {
  const tokens = [];
  for (let i = 1; i <= NUM_USERS; i++) {
    const token = login(username(i));
    if (token) {
      tokens.push(token);
    }
  }
  if (tokens.length === 0) {
    throw new Error(`没有任何 bench 账号能登录(seed 跑了吗?前缀=${PREFIX}?API 带 RATELIMIT_DISABLED=1 了吗?)`);
  }
  console.log(`setup: ${tokens.length}/${NUM_USERS} 个账号登录成功`);
  return { tokens };
}

export default function (data) {
  // 按 VU 轮换账号,让点赞分散到多个用户,大幅降低重复点赞概率。
  const token = data.tokens[(__VU - 1) % data.tokens.length];
  const id = Math.floor(Math.random() * (MAXID - MINID + 1)) + MINID;

  const res = http.post(`${BASE}/like/like`, JSON.stringify({ video_id: id }), {
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
  });

  if (res.status === 200) {
    likesNew.add(1);
  } else if (res.status === 409) {
    likesDup.add(1);
  }
  // 200(新点赞)与 409(重复,幂等)都算正常;其余(尤其 5xx)才是真问题。
  check(res, { 'ok or conflict': (r) => r.status === 200 || r.status === 409 });
}
