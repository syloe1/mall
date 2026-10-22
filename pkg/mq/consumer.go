package mq

import (
	"context"
	"encoding/json"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type Consumer struct {
	ch *amqp.Channel
}

type Handler func(context.Context, Message) error

func NewConsumer(r *RabbitMQ) (*Consumer, error) {
	ch, err := r.Channel()
	if err != nil {
		return nil, err
	}
	return &Consumer{ch: ch}, nil
}

func (c *Consumer) Start(queue string, prefetch int, handler Handler) error {
	/*
		c.ch.Qos(prefetch,0, false)
		一次只拿 prefetch 条消息
	*/
	if err := c.ch.Qos(prefetch, 0, false); err != nil {
		return err
	}

	msgs, err := c.ch.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	//开启协程异步消费
	go func() {
		for d := range msgs {
			//链路追踪（Trace）
			ctx := otel.GetTextMapPropagator().Extract(context.Background(), amqpHeaderCarrier(d.Headers))
			ctx, span := otel.Tracer("mall/rabbitmq").Start(ctx, "rabbitmq consume",
				oteltrace.WithSpanKind(oteltrace.SpanKindConsumer),
				oteltrace.WithAttributes(
					attribute.String("messaging.system", "rabbitmq"),
					attribute.String("messaging.destination.name", queue),
					attribute.String("messaging.message.id", d.MessageId),
				),
			)

			var msg Message
			if err := json.Unmarshal(d.Body, &msg); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				span.End()
				_ = d.Nack(false, false) // 丢弃消息（避免死循环）
				continue
			}

			if err := handler(ctx, msg); err != nil {
				log.Printf("consume failed: %v", err)
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				span.End()
				_ = d.Nack(false, true)
				continue
			}

			span.End()
			_ = d.Ack(false) // 确认消息 → MQ 删除消息
		}
	}()

	return nil
}

func (c *Consumer) Close() error {
	return c.ch.Close()
}

//
//1. 下单
//2. 发 15 分钟延迟消息
//3. 进入延迟队列等待
//4. 时间到 → 死信转发
//5. 进入超时队列
//6. Consumer 监听 → 取消订单
