package service

import (
	"context"
	"mall/pkg/mq"
)

// RabbitProductProducer 商品事件生产者
// 把 outbox 里的商品事件发到 RabbitMQ（实现 ProductEventProducer 接口）
// 原来的 Kafka 版本在 pkg/kafkax，改造后换成 RabbitMQ，接口不变
type RabbitProductProducer struct {
	p *mq.Producer
}

func NewRabbitProductProducer(p *mq.Producer) *RabbitProductProducer {
	return &RabbitProductProducer{p: p}
}

// PublishProductEvent 把商品事件包装成项目统一的 mq.Message，发到商品交换机
// msgID 用事件ID，方便追踪、排查、去重
func (r *RabbitProductProducer) PublishProductEvent(ctx context.Context, event mq.ProductEvent) error {
	msg := mq.Message{
		MsgID:     event.EventID,
		Event:     event.EventType,
		BizID:     event.ProductID,
		CreatedAt: event.OccurredAt,
		Payload:   event,
	}
	return r.p.PublishJSON(ctx, mq.ProductRoutingKey, msg, msg.MsgID)
}
