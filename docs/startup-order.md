# 启动说明

## 依赖顺序

建议按这个顺序启动：

1. MySQL
2. Redis
3. RabbitMQ
4. Meilisearch
5. Web 服务
6. Worker 服务

原因很简单：

- Web 启动时会做数据库初始化和依赖注入
- Worker 负责把 outbox 推到 RabbitMQ，并消费商品事件同步到 Meilisearch
- 搜索接口已经切到 Meilisearch，所以 Meilisearch 不起来，搜索就不可用

## Web 和 Worker

Web 进程：

```bash
go run ./cmd
```

Worker 进程：

```bash
go run ./cmd/worker
```

Web 默认读取 `config/config.yaml`。

Worker 默认读取 `config/worker.yaml`，用于独立配置 Meilisearch、MySQL 和链路追踪服务名。

## 端口和地址

- Web: `http://localhost:3333`
- MySQL: `127.0.0.1:13306`
- Redis: `127.0.0.1:6379`
- RabbitMQ: `amqp://guest:guest@127.0.0.1:5672/`
- Meilisearch: `http://127.0.0.1:7700`（管理界面同端口，需带主密钥）

## 启动后怎么验证

1. 创建商品
2. 看数据库里是否写入 `product_outboxes`
3. 看 worker 日志是否消费成功
4. 用搜索接口验证 Meilisearch 已同步

## 失败处理

- outbox 发送 RabbitMQ 失败后，会标记为 `failed`
- 商品事件消费失败重试 3 次后，会写入死信表 `product_event_dead_letters`
- 搜索接口不会再回退 MySQL
