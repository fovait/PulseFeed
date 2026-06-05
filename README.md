# PulseFeed

短视频社区全栈项目，后端 Go + Gin，前端 React + TypeScript。支持视频发布、信息流、点赞评论、关注关系、实时通知和内容审核。

---

## 技术栈

| 层 | 技术 |
|---|---|
| 后端语言 | Go 1.26 |
| Web 框架 | Gin |
| ORM | GORM + MySQL 8 |
| 缓存 | Redis 7 |
| 消息队列 | RabbitMQ |
| 认证 | JWT (HS256) + Refresh Token |
| 前端 | React 18 + TypeScript + Vite |
| 路由 | React Router v6 |
| 样式 | Tailwind CSS |

---

## 项目结构

```
PulseFeed/
├── backend/
│   ├── cmd/
│   │   ├── main.go          # API 服务入口（端口 8080）
│   │   └── worker/          # Worker 进程入口（消费 MQ）
│   ├── configs/
│   │   └── config.yaml      # 配置文件
│   └── internal/
│       ├── account/         # 账号：注册、登录、改密、改名
│       ├── video/           # 视频：发布、删除、详情、分片上传
│       ├── feed/            # 信息流：最新流、关注流、热门流、标签流
│       ├── social/          # 社交：关注、取关、粉丝/关注列表
│       ├── event/           # 行为埋点：曝光、播放、完播、分享
│       ├── moderation/      # 内容审核：举报、管理员审核
│       ├── worker/          # 后台 Worker：点赞/评论/热度/通知/时间线
│       ├── middleware/
│       │   ├── jwt/         # JWT 鉴权中间件
│       │   ├── redis/       # Redis 客户端封装
│       │   └── rabbitmq/    # RabbitMQ 生产者封装
│       ├── auth/            # Token 生成/解析
│       ├── config/          # 配置加载
│       ├── db/              # 数据库连接 & 自动迁移
│       ├── http/            # 路由组装 & 后台任务启动
│       └── observability/   # pprof 性能分析服务器
└── frontend/
    └── src/
        ├── pages/           # 页面组件（11 个页面）
        ├── components/      # 公共组件
        ├── api/             # API 客户端封装
        ├── hooks/           # 自定义 Hook
        ├── utils/           # 工具函数
        └── types/           # TypeScript 类型定义
```

---

## 功能模块

### 账号
- 注册 / 登录 / 登出
- Access Token（24h）+ Refresh Token（7d）双 Token 机制
- 改用户名（同步刷新 Token）/ 改密码
- 更新个人简介和头像
- Redis 鉴权热缓存，Miss 时自动回源 MySQL 自愈

### 视频
- 普通上传（单次）+ 分片上传（5MB/片，支持断点续传）
- 自动封面抓帧（前端）
- 发布时 `#标签` 自动提取，写入 `video_tags` 表
- Outbox 模式保证视频发布事件投递时间线 MQ 的一致性

### 信息流
- **最新流**：Redis 全局时间线 ZSET + MySQL 冷数据兜底，热冷拼接
- **关注流**：查关注用户的最新视频，30s 短缓存防击穿
- **热门流**：每分钟热度窗口 ZSET，合并近 60 分钟快照，offset 分页
- **点赞榜**：按 `likes_count DESC` 游标分页
- **标签流**：按标签名分页

### 点赞 & 评论
- 点赞/取消点赞 → 优先发 MQ，Worker 异步落库；MQ 不可用时直接写 MySQL
- 评论发布/删除同上，评论删除自动回滚 `comments_count`
- `@mention` 自动解析，触发站内通知

### 社交
- 关注 / 取关（幂等设计）
- 粉丝列表 / 关注列表（公开，游客可查）
- 关注/取关后删除关注流缓存

### 实时通知
- SSE 长连接推送增量通知（`/notification/stream`）
- REST 拉取历史通知 + 批量已读

### 内容审核
- 用户举报视频/评论
- 管理员查看举报列表（可按状态过滤）
- 管理员审核：通过 / 隐藏 / 拒绝
- 审核结果影响推荐可见性过滤

### 行为埋点
- 视频曝光 / 开始播放 / 完播 / 分享
- 作者可查自己视频的数据指标

---

## 架构要点

```
请求
 │
 ▼
Gin Handler
 │  参数校验、鉴权、限流
 ▼
Service
 │  业务逻辑、缓存策略、MQ 发布
 ▼
Repository
 │  GORM 操作 MySQL
 ▼
MySQL / Redis / RabbitMQ
```

**缓存分层（Feed 模块）**
```
L1 进程内缓存（go-cache, 3s）
  └─ miss → L2 Redis（1h）
               └─ miss → L3 MySQL
                            └─ 异步回写 L2
```

**异步写入链路（以点赞为例）**
```
LikeService.Like()
  ├─ 发布到 LikeMQ（RabbitMQ）
  │     └─ LikeWorker 消费 → MySQL 事务（likes 表 + likes_count）
  └─ 发布到 PopularityMQ
        └─ PopularityWorker 消费 → Redis 热榜 ZSET
```

**防缓存击穿**：Redis 分布式锁 + double-check + singleflight

---

## 快速启动

### 依赖

- Go 1.22+
- Node.js 20+
- MySQL 8
- Redis 7（可选，无 Redis 时退化为直查 MySQL）
- RabbitMQ 3.x（可选，无 MQ 时退化为直写 MySQL）

### 配置

复制并修改配置文件：

```bash
cp backend/configs/config.yaml backend/configs/config.local.yaml
```

```yaml
database:
  host: localhost
  port: 3306
  user: root
  password: your_password
  dbname: feedsystem

redis:
  host: localhost
  port: 6379
  password: ""

rabbitmq:
  host: localhost
  port: 5672
  username: guest
  password: guest

jwt:
  secret: "your-production-secret"

moderation:
  admin_account_ids: [1]   # 管理员账号 ID 白名单
```

> 程序按以下顺序查找配置：`$CONFIG_PATH` 环境变量 → `configs/config.local.yaml` → `configs/config.yaml`

### 启动后端

```bash
# 仅启动 API 服务（端口 8080）
make run-api

# 仅启动 Worker（消费 MQ 消息）
make run-worker

# 同时启动 API + Worker
make run-all

# 运行测试
make test
```

### 启动前端

```bash
cd frontend
npm install
npm run dev    # 开发服务器，默认 http://localhost:5173
```

---

## API 概览

所有请求均为 `POST`，`Content-Type: application/json`。
需要鉴权的接口在 Header 中携带 `Authorization: Bearer <token>`。

| 模块 | 路径 | 鉴权 |
|---|---|---|
| 注册 | `/account/register` | 否 |
| 登录 | `/account/login` | 否 |
| 刷新 Token | `/account/refreshToken` | 否 |
| 改用户名 | `/account/rename` | 是 |
| 发布视频 | `/video/publish` | 是 |
| 删除视频 | `/video/delete` | 是 |
| 视频详情 | `/video/getDetail` | 否 |
| 最新流 | `/feed/listLatest` | 否（登录态补充 is_liked）|
| 关注流 | `/feed/listByFollowing` | 是 |
| 热门流 | `/feed/listByPopularity` | 否 |
| 标签流 | `/feed/listByTag` | 否 |
| 点赞 | `/like/like` | 是 |
| 发评论 | `/video/comment/publish` | 是 |
| 关注 | `/social/follow` | 是 |
| 通知流 | `/notification/stream` | 是（SSE）|
| 举报 | `/moderation/report` | 是 |
| 审核（管理员）| `/moderation/review` | 是 |
| 举报列表（管理员）| `/moderation/listReports` | 是 |

---

## 前端页面

| 路由 | 页面 |
|---|---|
| `/` | 视频信息流（最新/关注/热门/标签） |
| `/video/:id` | 单视频详情播放页 |
| `/profile` | 我的主页（发布/点赞双 Tab） |
| `/user/:id` | 他人主页 |
| `/publish` | 发布视频 |
| `/messages` | 私信列表 |
| `/notifications` | 通知中心 |
| `/search` | 搜索（用户/标签） |
| `/tag/:name` | 标签视频流 |
| `/follow/:id` | 粉丝/关注列表 |
| `/admin/moderation` | 管理员审核台 |

---

## 开发说明

**数据库自动迁移**：服务启动时自动执行 `AutoMigrate`，无需手动建表。

**pprof**：开发模式下 API 进程在 `:6060`、Worker 进程在 `:6061` 暴露 pprof 端点，可用 `go tool pprof` 分析性能。

**降级策略**：Redis / RabbitMQ 均设计为可选依赖，不可用时自动降级为直查/直写 MySQL，服务不中断。

---

## 参考

- [feedsystem_video_go](https://github.com/LeoninCS/feedsystem_video_go)
