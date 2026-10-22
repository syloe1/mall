package mq // 放在消息队列工具包

import "time"

// ProductEvent 商品变更事件
// 所有商品创建、修改、删除、上下架、库存变化，都用这个结构体发消息
// （原 kafkax.ProductEvent，传输层从 Kafka 换成 RabbitMQ 后归入 mq 包）
type ProductEvent struct {
	EventID    string    `json:"event_id"`    // 事件唯一ID（UUID，用来追踪这条消息）
	EventType  string    `json:"event_type"`  // 事件类型：created/updated/deleted/stock_changed 等
	ProductID  uint      `json:"product_id"`  // 商品ID（哪一个商品变了）
	OccurredAt time.Time `json:"occurred_at"` // 事件发生时间
}
