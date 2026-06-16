#!/usr/bin/env bash
# 压测矩阵驱动脚本。每个实验 = 一个"对照",凸显一个指标。
# 用法:
#   loadtest/run.sh seed         # 灌数据(只需一次)
#   loadtest/run.sh A            # 实验A 基线爬坡(listLatest)
#   loadtest/run.sh B            # 实验B 缓存命中 vs 冷兜底(成对)
#   loadtest/run.sh C            # 实验C 异步写吞吐(需先用 RATELIMIT_DISABLED=1 起 API)
#   loadtest/run.sh watch-mq     # 实时盯 RabbitMQ 队列积压(实验C 配合用)
#   loadtest/run.sh prof         # 抓 30s CPU profile
#
# 可调:BASE、ACCOUNTS、VIDEOS、MINID、MAXID 等环境变量。
set -euo pipefail

BASE="${BASE:-http://localhost:8080}"
K6="${K6:-k6}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
K6DIR="$ROOT/loadtest/k6"
MINID="${MINID:-1}"
MAXID="${MAXID:-2000}"

cmd="${1:-help}"
case "$cmd" in
  seed)
    ACCOUNTS="${ACCOUNTS:-100}"; VIDEOS="${VIDEOS:-2000}"
    echo ">> seeding: $ACCOUNTS accounts, $VIDEOS videos"
    ( cd "$ROOT/backend" && go run ./cmd/seed -accounts="$ACCOUNTS" -videos="$VIDEOS" )
    echo ">> 记下上面输出的 id 区间,作为 MINID/MAXID 传给后续实验"
    ;;

  A) # 基线爬坡:找 QPS 上限与延迟拐点
    BASE="$BASE" TARGET=latest "$K6" run "$K6DIR/read.js"
    ;;

  B) # 缓存命中 vs 冷兜底:对照看 p99 差值
    echo "=== B1: 缓存友好(首页,SKEW=0) ==="
    BASE="$BASE" TARGET=latest SKEW=0 "$K6" run "$K6DIR/read.js"
    echo "=== B2: 缓存打散(随机 before_time,SKEW=30d) ==="
    BASE="$BASE" TARGET=latest SKEW=2592000 "$K6" run "$K6DIR/read.js"
    echo ">> 对比两次 p95/p99:差值≈缓存为你省下的延迟"
    ;;

  C) # 异步写吞吐:另开一个终端跑 'run.sh watch-mq' 同步观察队列
    echo ">> 确认 API 是以 RATELIMIT_DISABLED=1 启动的,否则会被限流"
    BASE="$BASE" MINID="$MINID" MAXID="$MAXID" "$K6" run "$K6DIR/write.js"
    echo ">> 压测停止后,看 watch-mq 队列从峰值排空到 0 的耗时 ÷ 消息数 = Worker 消费速率"
    ;;

  watch-mq) # 实时队列积压曲线(需 watch:brew install watch)
    # -q 静默 "Listing queues..." 前导行;column -t 把制表符分隔输出对齐成整齐表格
    watch -n1 'rabbitmqctl -q list_queues name messages messages_ready messages_unacknowledged | column -t'
    ;;

  prof) # 压测进行中抓火焰图素材
    PORT="${PORT:-6060}"; SEC="${SEC:-30}"
    go tool pprof "http://localhost:${PORT}/debug/pprof/profile?seconds=${SEC}"
    ;;

  *)
    grep -E '^#( |$)' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
    ;;
esac
