package mq

import amqp "github.com/rabbitmq/amqp091-go"

type amqpHeaderCarrier amqp.Table

// 从 RabbitMQ 消息头里读取 trace 信息（traceId、spanId）
func (c amqpHeaderCarrier) Get(key string) string {
	value, ok := c[key]
	if !ok || value == nil {
		return ""
	}
	if str, ok := value.(string); ok {
		return str
	}
	return ""
}

func (c amqpHeaderCarrier) Set(key string, value string) {
	c[key] = value
}

func (c amqpHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for key := range c {
		keys = append(keys, key)
	}
	return keys
}

//生产者发送消息
//↓
//把 traceId、spanId 放进 amqpHeaderCarrier
//↓
//自动写入 RabbitMQ 消息头
//↓
//消费者收到消息
//↓
//用 amqpHeaderCarrier 读取 traceId、spanId
//↓
//整条链路就连起来了！
