package mq

import amqp "github.com/rabbitmq/amqp091-go"

// 商品事件队列（outbox → 商品索引同步）
// 交换机与队列都持久化，重启不丢
const (
	ProductExchange   = "mall.product.exchange"
	ProductQueue      = "mall.product.queue"
	ProductRoutingKey = "product.changed"
)

// DeclareProductQueues 声明商品事件需要的交换机、队列、绑定关系
// 调用方：worker 启动时声明一次
func DeclareProductQueues(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(ProductExchange, "direct", true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(ProductQueue, true, false, false, false, nil); err != nil {
		return err
	}
	return ch.QueueBind(ProductQueue, ProductRoutingKey, ProductExchange, false, nil)
}
