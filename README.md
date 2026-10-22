# Mall · Go 电商后端

一个用于求职作品集的**全栈电商项目后端**：Go + Gin 分层架构，覆盖 JWT 双 Token 认证、购物车、订单事务、支付幂等、秒杀（Redis Lua 原子扣库存 + MQ 异步落单）、Outbox → RabbitMQ → Meilisearch 搜索同步、死信表审计、限流与链路追踪。

- 🌐 **在线演示**：<https://mall.syloe1.xyz>（演示账号见下方）
- 🖥 **前端仓库**：<https://github.com/syloe1/mall-web>（Vue 3 + TypeScript）
- 👤 **作者**：李俊哲 · 后端开发工程师（Go / C++）· [作品集](https://syloe1.xyz)
- 📄 **项目说明页**：<https://mall.syloe1.xyz/about>（架构图 / 六条核心链路 / 难点取舍 / 压测数据 / 演示账号）

![Mall 商城首页](docs/screenshots/home.png)

## 目录

- [项目亮点](#项目亮点)
- [系统架构](#系统架构)
- [核心链路](#核心链路)
- [技术栈](#技术栈)
- [快速开始](#快速开始)
- [演示账号](#演示账号)
- [安全基线](#安全基线)
- [API 概览](#api-概览)
- [测试](#测试)
- [目录结构](#目录结构)
- [部署](#部署)
- [取舍与后续](#取舍与后续)
- [License](#license)

## 项目亮点

| 能力 | 实现 |
|---|---|
| 认证 | JWT access token（15min，内存）+ refresh token（7d，HttpOnly Cookie）；jti 黑名单、登出吊销、单飞刷新 |
| 账号安全 | 服务端密码策略（≥8 位 + 字母数字 + 弱口令黑名单）、bcrypt cost 12、dummy hash 防用户名枚举、登录 IP + 账号双维度限流 |
| 订单 | 事务内校验/扣减库存 + 订单/明细快照落库 + RabbitMQ 延迟消息 30 分钟超时关单 |
| 支付 | 支付单号唯一索引 + 状态机保证幂等；余额 AES 加密 + bcrypt 支付密码；模拟支付仅在 `DEMO_MODE` 挂载 |
| 秒杀 | Redis Lua 单脚本原子扣库存（防超卖）、Redis 分布式锁（防重复提交）、令牌桶限流、MQ 异步落真实订单、活动状态按时间窗口实时派生 |
| 搜索 | MySQL Outbox 同事务 → RabbitMQ → Meilisearch；失败重试进死信表；后台「重建索引」全量对账 |
| 可观测 | `X-Request-ID` 全链路、slog 结构化访问日志、OpenTelemetry 埋点（HTTP / GORM / Redis / MQ） |
| 工程 | Handler → Service → DAO 分层 + 接口注入；DAO 用 SQLite 内存库测真实 SQL，Service 用 mock DAO + miniredis；246 个测试函数，`-race` 通过 |
| 部署 | Web / Worker 双进程，systemd 守护；Nginx 静态托管 + 安全响应头 + API 反代；Cloudflare Tunnel（2C2G 云服务器） |

## 系统架构

```
┌──────────────── 浏览器（Vue 3 + TypeScript SPA）────────────────┐
│  路由守卫 · Pinia · axios 拦截器（401 单飞刷新）· 无障碍 / SEO   │
└───────────────────────────────┬─────────────────────────────────┘
                                │ HTTPS · Cloudflare Tunnel
┌───────────────────────────────▼─────────────────────────────────┐
│ Nginx：静态托管 + gzip + 安全响应头 + SPA 回退 + /api 反代       │
└───────────────────────────────┬─────────────────────────────────┘
                                │
        ┌───────────────────────▼────────────────────────┐
        │             Go Web（Gin，/api/v1）              │
        │  JWT 双 Token · 登录限流 · X-Request-ID 全链路  │
        │  Handler → Service → DAO（接口注入，可 mock）   │
        └───┬────────────────┬────────────────┬──────────┘
            │                │                │
      ┌─────▼──────┐   ┌─────▼──────┐   ┌─────▼───────┐
      │   MySQL    │   │   Redis    │   │  RabbitMQ   │
      │  15 张表   │   │ 库存/会话  │   │ 延迟/事件   │
      └────────────┘   └────────────┘   └─────┬───────┘
                                              │
        ┌─────────────────────────────────────▼──────────┐
        │        Go Worker（独立进程，异步消费者）        │
        │  Outbox Publisher → 商品事件 → Meilisearch      │
        │  秒杀消费 → 落真实订单 · 失败写死信表审计       │
        └─────────────────────────────────────────────────┘
```

- **Web**：只处理 HTTP 请求（订单超时消费者也在这个进程）
- **Worker**：Outbox 轮询发布、商品事件→Meilisearch、秒杀消息→真实订单
- 业务表与 outbox 同事务提交，最终一致；失败进 `product_event_dead_letters`，后台可查

## 核心链路

1. **登录认证**：注册（密码策略 + bcrypt 12）→ 登录（限流 + dummy hash + 统一 401）→ access 内存 / refresh HttpOnly Cookie → 401 单飞刷新 → 登出 jti 黑名单
2. **购物车**：加购校验库存 → 勾选/全选/改数量/清空（服务端持久化）
3. **下单**：事务扣库存 + 订单快照 + 30 分钟延迟关单，超时自动取消并回滚库存
4. **支付**：幂等支付单 + 状态机；余额支付走支付密码 + AES 余额；模拟支付受 `DEMO_MODE` 开关控制
5. **秒杀**：Redis Lua 原子扣库存 → MQ 异步落单 → 失败回滚；活动状态由 `start_at/end_at` 实时派生（`upcoming/running/ended`），已结束返回 410
6. **搜索**：商品写 MySQL + outbox 同事务 → Worker 投递 RabbitMQ → 消费写 Meilisearch → 失败进死信表；后台可全量重建索引

## 技术栈

| 层级 | 选型 |
|---|---|
| 语言 / 框架 | Go 1.25、Gin、GORM |
| 数据库 | MySQL 8（15 张表，GORM AutoMigrate） |
| 缓存 / 并发 | Redis 7（refresh token、秒杀库存 Lua、分布式锁、登录限流） |
| 消息队列 | RabbitMQ（延迟队列 / 秒杀异步 / Outbox 商品事件） |
| 搜索 | Meilisearch（轻量、2C2G 友好，替代 Elasticsearch） |
| 可观测 | slog、X-Request-ID、OpenTelemetry |
| 测试 | testify、miniredis、SQLite 内存库、k6 |
| 部署 | Docker Compose（本地）/ systemd + Nginx + Cloudflare Tunnel（线上） |
| 文档 | Swagger（swaggo 注解） |

## 快速开始

### 方式一：Docker Compose（推荐体验）

```bash
cp .env.example .env          # 填入本地开发密钥（JWT_SECRET / AES_KEY 等）
docker compose up -d
curl http://localhost:18888/api/v1/ping
# Swagger: http://localhost:18888/swagger/index.html
```

7 个服务：MySQL / Redis / RabbitMQ / Meilisearch / Web / Worker / Nginx。

### 方式二：本地进程 + 容器中间件

```bash
# 依赖：MySQL 13306 / Redis 6379 / RabbitMQ 5672 / Meilisearch 7700（可用 docker compose 只起中间件）
go run ./cmd                 # Web API :3333
go run ./cmd/worker          # Worker（异步消费者）
go run ./cmd/seed -reset     # 演示数据：分类 / demo+admin 账号 / 滚动秒杀活动 / 商品图 URL
```

> `cmd/seed` 需要 `AES_KEY`（余额字段 AES 加密）。生产环境由 systemd/环境变量注入。
> 演示数据每日重置：服务器 cron 每天执行 `mall-seed -reset`。

## 演示账号

| 账号 | 密码 | 角色 | 说明 |
|---|---|---|---|
| `demo` | `Demo@123456` | user | 完整体验浏览、加购、下单、模拟支付 |
| `admin` | `Admin@123456` | admin | 管理后台：商品管理 / 死信队列 / 重建搜索索引 |

演示环境数据每日重置，模拟支付不产生真实扣款。

## 安全基线

- **密码**：服务端强制 ≥8 位 + 字母数字 + 弱口令黑名单；bcrypt cost 12；旧密码校验失败统一文案
- **登录**：IP + 账号双维度 5 次/分钟限流，超限 `429 + Retry-After: 60`；用户不存在与密码错误统一 `401`，并用 dummy hash 抹平响应时间差
- **Token**：access token 只存前端内存；refresh token 放 `HttpOnly + SameSite=Lax + Secure(HTTPS)` Cookie，支持 jti 黑名单与登出吊销
- **CORS**：环境变量白名单，命中才回显 `Access-Control-Allow-Origin` 并带 `Vary: Origin`；未知 Origin 预检 403；默认不开启 `Allow-Credentials`
- **错误处理**：统一错误中间件；参数绑定错误 400、未知路由 JSON 404、panic 统一 500；对外只返回 `{code,msg,request_id}`，内部错误只写日志
- **HTTP 安全头**（Nginx）：HSTS、CSP、X-Content-Type-Options、X-Frame-Options、Referrer-Policy、Permissions-Policy、COOP；鉴权接口 `Cache-Control: no-store`
- **配置**：密钥只从环境变量注入，仓库不包含 `.env`；`JWT_SECRET` 为空时服务拒绝启动
- **演示开关**：`DEMO_MODE` 控制模拟支付接口是否挂载；`/admin` 路由与后台接口双层鉴权

## API 概览

完整接口见项目说明页或本地 Swagger。统一响应：`{"code":200,"msg":"success","request_id":"...","data":...}`。

| 模块 | 主要接口 |
|---|---|
| 系统 | `GET /api/v1/ping`、`GET /api/v1/meta`、`GET /api/v1/categories` |
| 用户 | `POST /users/register`、`POST /users/login`、`POST /users/refresh-token`、`POST /users/logout`、`GET/PUT /users/me`、`PUT /users/me/password`、`PUT /users/me/pay-password`、`GET /users/me/amount` |
| 商品 | `GET /products`、`GET /products/search`、`GET /products/:id`、`GET /products/categories` |
| 地址 / 收藏 | `/users/addresses` CRUD + 默认地址；`/users/favorite/folders`、`/users/favorite/items`、`/users/favorite/check` |
| 购物车 | `/users/cart/items` CRUD、`PUT /users/cart/items/checked`、`PUT /users/cart/checked`、`DELETE /users/cart` |
| 订单 / 支付 | `POST /users/orders/cart`、`POST /users/orders/buy-now`、`GET /users/orders`、`POST /users/payments`、`GET /users/payments/detail`、`POST /users/payments/mock/success`（DEMO_MODE） |
| 秒杀 | `GET /seckill/activities`、`POST /seckill/activities/:id/buy`、`GET /seckill/activities/:id/my-result` |
| 管理 | `/admin/products` CRUD、`POST /admin/products/reindex`、`POST /admin/seckill/activities/init-stock`、`GET /admin/dead-letters/products` |

## 测试

```bash
go test ./... -count=1                 # 全部单测（246 个测试函数）
go test -race ./service ./middleware ./dao
go vet ./...
```

| 层级 | 覆盖率（`go test -cover ./...`） | 说明 |
|---|---|---|
| DAO | 69.3% | SQLite 内存库 + AutoMigrate，验证真实 SQL |
| Service | 76.7% | testify mock DAO + miniredis，覆盖成功/失败/幂等/回滚 |
| Middleware | 40.8% | JWT、Admin、CORS 白名单、登录限流、分页校验 |
| pkg | 90%+ | aes / ratelimit / retryx / tracex |

压测脚本位于 `k6/scenarios/`：

```bash
k6 run -e SCENARIO=products -e BASE=https://mall.syloe1.xyz/api/v1 k6/scenarios/public_read.js
```


## 性能（线上实测）

| 场景 | 方式 | 并发 | 吞吐 | P95 | 错误率 |
|---|---|---|---|---|---|
| 商品列表（公网全链路） | k6 | 100 VU | 116.9 req/s | 724 ms | 0.08% |
| 混合只读（列表+详情+搜索+分类） | k6 | 100 VU | 122.9 req/s | 576 ms | 0.03% |
| 商品列表（服务器本机，含 Nginx） | ab | c=100 | 255.9 req/s | 588 ms | 0% |
| 商品搜索（服务器本机） | ab | c=100 | 457.0 req/s | 259 ms | 0% |
| 分类字典（服务器本机） | ab | c=100 | 923.2 req/s | 213 ms | 0% |

> 环境：阿里云 2C2G（Ubuntu 22.04 / Nginx 1.18 / Go 1.25）+ Cloudflare Tunnel。
> 公网 k6 含真实网络与 TLS 开销；本机 ab 衡量服务端自身吞吐。
> 压测中修复了 Go 连接池（100 × Web/Worker）打满 MySQL `max_connections` 的 Error 1040：
> 单进程连接池下调至 60、MySQL 上限调到 200 后失败率归零。

## 目录结构

```
cmd/               Web / Worker / seed 三个入口
config/            配置加载（支持环境变量覆盖 + 安全配置）
container/         依赖注入容器
dao/               数据访问层（接口 + GORM 实现 + 测试）
database/          AutoMigrate
dto/               请求 / 响应 DTO
handler/           HTTP Handler（Swagger 注解 + 参数校验）
middleware/        JWT / Admin / CORS / 限流 / RequestID / Recovery / 日志 / Tracing
mock/              Service 单测 mock
model/             GORM 模型（含分类字典、秒杀活动状态派生）
pkg/               aes / bootstrap / core(Response) / lock / logx / mq / ratelimit / retryx / seckill / tracex
router/            路由注册（统一 404/405、公开/鉴权/admin 分组）
service/           业务逻辑 + 单测
k6/                压测脚本
nginx/             本地 Docker 环境 Nginx 配置
static/            静态资源（头像等）
```

## 部署

线上为**非 Docker** 部署（2C2G 云服务器）：

```bash
# 1. 编译 Linux 二进制
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o mall-web ./cmd
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o mall-worker ./cmd/worker
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o mall-seed ./cmd/seed

# 2. systemd 托管（JWT_SECRET / AES_KEY 由 Environment 注入）
# 3. Nginx 托管前端静态文件并把 /api、/static 反代到 127.0.0.1:3002
# 4. Cloudflare Tunnel 暴露域名（TLS 在边缘终止）
```

前端 Nginx 配置（安全响应头 / CSP / SPA 回退 / 404 状态码 / 缓存策略）见前端仓库 `mall-nginx.conf`。

## 取舍与后续

- 搜索切 Meilisearch 后**不回退 MySQL**：写入路径简单，索引漂移用 Outbox + 后台 reindex 兜底
- 秒杀预扣 Redis 库存，与 MySQL 有短暂不一致窗口，用失败回滚 + 消费幂等收敛
- 商品图使用自托管 SVG 插画（脚本生成，含商品名），不依赖第三方图床；换真实图只需改图片 URL
- 待补：订单/支付 DTO 化（当前部分接口仍直接序列化 ORM 模型）、OpenTelemetry 接入 Jaeger 后端、更多 k6 场景（下单/支付链路）

## License

[MIT](./LICENSE)
