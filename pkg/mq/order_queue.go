package mq

import amqp "github.com/rabbitmq/amqp091-go"

// 交换机
const (
	// 订单延迟交换机：存放“等待超时”的消息
	//接收：刚创建的订单消息
	OrderDelayExchange = "mall.order.delay.exchange"

	// 订单事件交换机：存放“已经超时”的消息
	OrderEventExchange = "mall.order.event.exchange"

	// 队列
	// 延迟队列：订单刚创建，进入这里等待超时
	OrderDelayQueue = "mall.order.delay.queue"

	// 超时队列：等待时间到了，消息转发到这里，执行取消订单
	OrderTimeoutQueue = "mall.order.timeout.queue"

	// 路由键（消息的“门牌号”）
	// 订单创建 → 进入延迟队列的路由key
	OrderDelayRoutingKey = "order.created.delay"

	// 延迟时间到 → 进入超时队列的路由key
	OrderTimeoutRoutingKey = "order.timeout"

	SeckillExchange   = "mall.seckill.exchange"
	SeckillOrderQueue = "mall.seckill.order.queue"
	SeckillOrderKey   = "seckill.order.create"
)

func DeclareSeckillQueues(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(SeckillExchange, "direct", true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(SeckillOrderQueue, true, false, false, false, nil); err != nil {
		return err
	}
	return ch.QueueBind(SeckillOrderQueue, SeckillOrderKey, SeckillExchange, false, nil)
}
