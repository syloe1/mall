package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type Producer struct {
	ch       *amqp.Channel
	exchange string
}

func NewProducer(r *RabbitMQ, exchange string) (*Producer, error) {
	ch, err := r.Channel()
	if err != nil {
		return nil, err
	}
	return &Producer{ch: ch, exchange: exchange}, nil
}

// 发普通JSON消息
func (p *Producer) PublishJSON(ctx context.Context, routingKey string, v any, msgID string) error {
	return p.publish(ctx, routingKey, v, msgID, 0)
}

// 发延迟JSON消息（订单超时用这个！）
func (p *Producer) PublishDelayJSON(ctx context.Context, routingKey string, v any, msgID string, delay time.Duration) error {
	return p.publish(ctx, routingKey, v, msgID, delay)
}

func (p *Producer) publish(ctx context.Context, routingKey string, v any, msgID string, delay time.Duration) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}

	spanName := "rabbitmq publish"
	if delay > 0 {
		spanName = "rabbitmq publish delay"
	}
	//链路追踪（opentelemetry）
	ctx, span := otel.Tracer("mall/rabbitmq").Start(ctx, spanName,
		oteltrace.WithSpanKind(oteltrace.SpanKindProducer),
		oteltrace.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.destination.name", p.exchange),
			attribute.String("messaging.rabbitmq.routing_key", routingKey),
			attribute.String("messaging.message.id", msgID),
			attribute.Int64("messaging.delay_ms", delay.Milliseconds()),
		),
	)
	defer span.End()
	//把追踪信息注入消息头
	headers := amqp.Table{}
	otel.GetTextMapPropagator().Inject(ctx, amqpHeaderCarrier(headers))

	publishing := amqp.Publishing{
		ContentType:  "application/json", // JSON格式
		DeliveryMode: amqp.Persistent,    // 持久化（重启不丢）
		MessageId:    msgID,              // 消息唯一ID
		Timestamp:    time.Now(),         // 时间
		Body:         body,               // 消息内容
		Headers:      headers,            // 追踪头
	}
	if delay > 0 {
		publishing.Expiration = fmt.Sprintf("%d", delay.Milliseconds())
	}

	err = p.ch.PublishWithContext(ctx, p.exchange, routingKey, false, false, publishing)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

func (p *Producer) Close() error {
	return p.ch.Close()
}
