package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"mall/dao"
	"mall/model"
	"mall/pkg/mq"
	"mall/pkg/retryx"
	"time"
)

type ProductOutboxPublisher struct {
	outboxDB     dao.ProductOutboxDB     // 发件箱表（核心！）
	deadLetterDB dao.ProductDeadLetterDB // 死信表
	producer     ProductEventProducer    // RabbitMQ消息生产者
	interval     time.Duration           // 轮询间隔
	limit        int                     // 一次拉多少条
}
type ProductEventProducer interface {
	PublishProductEvent(ctx context.Context, event mq.ProductEvent) error
}

func NewProductOutboxPublisher(outboxDB dao.ProductOutboxDB, deadLetterDB dao.ProductDeadLetterDB, producer ProductEventProducer) *ProductOutboxPublisher {
	return &ProductOutboxPublisher{
		outboxDB:     outboxDB,
		deadLetterDB: deadLetterDB,
		producer:     producer,
		interval:     2 * time.Second,
		limit:        50,
	}
}

// 启动发件箱定时发布器（一直运行，不退出）
func (p *ProductOutboxPublisher) Start(ctx context.Context) {
	// 安全检查：依赖不全直接退出，防止崩溃
	if p == nil || p.outboxDB == nil || p.producer == nil {
		return
	}

	// 创建定时器：每隔 interval（比如 100ms）触发一次
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop() // 退出时关闭定时器，避免泄漏

	// 死循环：一直运行
	for {
		// 检查是否收到退出信号（服务关闭）
		select {
		case <-ctx.Done():
			return
		default:
			// 执行一次：从发件箱取消息 → 发到 RabbitMQ
			p.publishOnce(ctx)
		}
		// 执行完default → 代码**直接走到第二个阻塞select**
		// 等待下一个周期（等 ticker 或退出信号）
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// 执行一次：从发件箱取待发送消息 → 发到 RabbitMQ → 标记已发送/失败
func (p *ProductOutboxPublisher) publishOnce(ctx context.Context) {
	// 1. 从 outbox 表查询【待发送】的消息（一次最多取 limit 条）
	events, err := p.outboxDB.ListPending(p.limit)
	if err != nil {
		slog.Error("list product outbox failed", "error", err)
		return
	}

	// 2. 遍历每一条待发送消息
	for _, outbox := range events {
		// 组装成要发送到 RabbitMQ 的事件结构
		event := mq.ProductEvent{
			EventID:    outbox.EventID,
			EventType:  outbox.EventType,
			ProductID:  outbox.ProductID,
			OccurredAt: outbox.CreatedAt,
		}

		// 3. 发送消息（失败自动重试 3 次，间隔 200ms）
		err := retryx.Do(ctx, 3, 200*time.Millisecond, func() error {
			return p.producer.PublishProductEvent(ctx, event)
		})

		// 4. 如果重试 3 次还是失败
		if err != nil {
			// 标记发件箱记录为【发送失败】
			_ = p.outboxDB.MarkFailed(outbox.ID, err.Error())
			// 记录到死信表，方便后续人工处理
			p.recordDeadLetter(ctx, event, model.ProductDeadLetterPublisher, err, outbox.RetryCount+1)
			slog.Error("publish product outbox failed", "outbox_id", outbox.ID, "product_id", outbox.ProductID, "error", err)
			continue
		}

		// 5. 发送成功！标记发件箱记录为【已发送】
		if err := p.outboxDB.MarkPublished(outbox.ID); err != nil {
			slog.Error("mark product outbox published failed", "outbox_id", outbox.ID, "error", err)
			continue
		}
		slog.Info("published product event", "outbox_id", outbox.ID, "event_type", outbox.EventType, "product_id", outbox.ProductID)
	}
}

// 发布消息失败 → 写入死信表（人工后续处理）
func (p *ProductOutboxPublisher) recordDeadLetter(
	ctx context.Context,
	event mq.ProductEvent, // 商品事件
	source string, // 来源：这里是 publisher（发送器）
	err error, // 失败原因
	retryCount int, // 已经重试了几次
) {
	// 没有死信DB就直接返回
	if p.deadLetterDB == nil {
		return
	}

	// 把事件转成 JSON 字符串（完整保存原始消息）
	payload, _ := json.Marshal(event)

	// 创建一条死信记录，存入数据库
	_ = p.deadLetterDB.Create(&model.ProductEventDeadLetter{
		EventID:      event.EventID,   // 事件唯一ID（追踪用）
		EventType:    event.EventType, // 创建/更新/删除
		ProductID:    event.ProductID, // 商品ID
		Source:       source,          // 来源：publisher（发送失败）
		RetryCount:   retryCount,      // 重试次数
		ErrorMessage: err.Error(),     // 错误信息（为什么发不出去）
		Payload:      string(payload), // 完整消息体
	})
}
