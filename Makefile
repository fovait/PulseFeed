.PHONY: run-api run-worker run-all test

# 仅启动 API 服务（端口 8080）
run-api:
	cd backend && go run ./cmd

# 仅启动后台 Worker（消费 MQ 消息）
run-worker:
	cd backend && go run ./cmd/worker

# 同时启动 API + Worker（前台运行两个进程）
run-all:
	cd backend && go run ./cmd & go run ./cmd/worker

# 运行全部测试
test:
	cd backend && go test ./...
