package mq

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQ struct {
	conn *amqp.Connection
}

// 只管连接， 关连接， 开channel
func NewRabbitMQ(url string) (*RabbitMQ, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	return &RabbitMQ{conn: conn}, nil
}

func (r *RabbitMQ) Channel() (*amqp.Channel, error) {
	return r.conn.Channel()
}

func (r *RabbitMQ) Close() error {
	return r.conn.Close()
}

func MustDeclareExchange(ch *amqp.Channel, name string) error {
	return ch.ExchangeDeclare(
		name,     // 交换机名字
		"direct", // 交换机类型：直连模式
		true,     // 持久化
		false,    // 自动删除
		false,    // 独占
		false,    // 不等待服务器返回
		nil,      // 额外参数
	)
}
func MustDeclareQueue(ch *amqp.Channel, name string) (amqp.Queue, error) {
	return ch.QueueDeclare(
		name,  // 队列名字
		true,  // 持久化
		false, // 自动删除
		false, // 独占队列
		false, // 不等待
		nil,   // 额外参数
	)
}

// 声明订单超时需要的所有交换机、队列、绑定关系
// 延迟队列 → 过期 → 死信 → 真正消费队列
func DeclareOrderTimeoutQueues(ch *amqp.Channel) error {

	// 1. 声明【延迟交换机】
	if err := ch.ExchangeDeclare(OrderDelayExchange, "direct", true, false, false, false, nil); err != nil {
		return err
	}

	// 2. 声明【事件交换机（超时用）】
	if err := ch.ExchangeDeclare(
		OrderEventExchange,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return err
	}

	// 3. 声明【延迟队列】—— 最关键！带死信配置
	_, err := ch.QueueDeclare(
		OrderDelayQueue,
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    OrderEventExchange,     // 消息过期后转发到这个交换机
			"x-dead-letter-routing-key": OrderTimeoutRoutingKey, // 用这个路由键转发
		},
	)
	if err != nil {
		return err
	}

	// 4. 绑定：延迟队列 ←→ 延迟交换机
	if err := ch.QueueBind(OrderDelayQueue, OrderDelayRoutingKey, OrderDelayExchange, false, nil); err != nil {
		return err
	}

	// 5. 声明【超时队列】（真正被消费的队列）
	_, err = ch.QueueDeclare(OrderTimeoutQueue, true, false, false, false, nil)
	if err != nil {
		return err
	}

	// 6. 绑定：超时队列 ←→ 事件交换机
	return ch.QueueBind(OrderTimeoutQueue,
		OrderTimeoutRoutingKey,
		OrderEventExchange,
		false,
		nil)
}
func MustBindQueue(ch *amqp.Channel, queue, exchange, routingKey string) error {
	return ch.QueueBind(
		queue,      // 1. 队列名：要绑定的队列
		routingKey, // 2. 路由键：消息匹配规则
		exchange,   // 3. 交换机名：绑到哪个交换机
		false,      // 4. noWait = 是否不等待服务端确认
		nil,        // 5. 额外参数
	)
}

//1. 下单 → 发送消息到【延迟交换机】
//2. 路由到【延迟队列】（不消费，只等待）
//3. 等待超时时间（比如15分钟）
//4. 消息变成死信 → 自动转发到【死信交换机】
//5. 路由到【超时队列】
//6. 消费者监听 → 执行订单取消
