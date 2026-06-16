# PulseFeed 压测套件

用「成对的对照实验」让指标自己说话:固定其他变量、只改一个开关,两次结果的**差值**就是某个子系统的真实贡献。配合 API 的 `/metrics`(Prometheus)+ pprof 一起看。

## 目录

```
loadtest/
├── run.sh          # 矩阵驱动:seed / A / B / C / watch-mq / prof
└── k6/
    ├── read.js     # 读路径:listLatest / listByPopularity / getDetail
    └── write.js    # 写路径:登录 → 异步点赞
backend/cmd/seed/   # Go 数据 seeder(复用项目 db/model,密码哈希一致)
```

## 前置

1. 装 [k6](https://k6.io/docs/get-started/installation/):`brew install k6`
2. MySQL / Redis / RabbitMQ 起好(压异步写需要 MQ + Worker)
3. 灌数据(只需一次):
   ```bash
   loadtest/run.sh seed                  # 默认 100 账号 / 2000 视频
   ACCOUNTS=200 VIDEOS=5000 loadtest/run.sh seed
   ```
   记下输出的 **id 区间**,作为 `MINID/MAXID` 传给后续实验。

## 实验矩阵

| 实验 | 命令 | 对照变量 | 凸显的指标 |
|---|---|---|---|
| **A** 基线爬坡 | `run.sh A` | VU 50→500 | QPS 上限 + 延迟拐点(knee) |
| **B** 缓存命中 vs 冷兜底 | `run.sh B` | `SKEW=0` vs `SKEW=30d` | 缓存命中率价值(p99 差值) |
| **C** 异步写吞吐 | `run.sh C` + `run.sh watch-mq` | HTTP 平 / 队列涨 | MQ 解耦 + Worker 消费滞后 |
| **D** 限流开/关 | A/C 各跑两遍 | `RATELIMIT_DISABLED` | 限流那一跳 Redis 开销 |
| **E** 降级演练 | 压测中 `redis-cli shutdown` | Redis 在/不在 | 可用性不中断 + 降级延迟跳变 |
| **F** 浸泡 soak | 改 `read.js` 恒定 30min | 时间 | 内存/goroutine 泄漏 |

## 关键开关:限流

写接口默认有限流(like 30/min、login 10/min),单账号压测会被 429 限死。压**写路径**前用开关旁路:

```bash
cd backend && RATELIMIT_DISABLED=1 go run ./cmd     # 仅本地压测用
```

读路径(listLatest/popularity/getDetail)无限流,无需此开关。

## 一边压一边看指标

```bash
# 服务端 QPS / p99 / 错误率(配 Prometheus 抓取 :8080/metrics,见根 README)
# 压测中抓 CPU 火焰图素材:
loadtest/run.sh prof                  # = go tool pprof :6060/debug/pprof/profile?seconds=30

# 实验C 必看:队列积压曲线
loadtest/run.sh watch-mq

# Redis 缓存命中率(实验B 佐证)
redis-cli -a 123456 INFO stats | grep keyspace
```

## 怎么读结果

- **A**:p99 突然非线性翘起的那个 VU = 容量上限;翘起前的 QPS 才是可用容量。
- **B**:B2 比 B1 的 p99 高出的部分 = 缓存省下的延迟;差越大缓存越值。
- **C**:HTTP p99 平稳但队列暴涨且排空慢 → Worker 是瓶颈(加并发 / 提高 prefetch)。
- **E**:杀 Redis 后只变慢、不出 5xx → 降级达标;若直接 5xx 说明降级有 bug。
