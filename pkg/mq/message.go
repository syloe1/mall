package mq // 放在消息队列工具包

import "time" // 时间字段要用

// Message 是项目统一的 MQ 消息结构体
// 所有事件（订单创建、支付、超时、商品变动…）都用它发
type Message struct {
	MsgID     string    `json:"msg_id"`            // 消息唯一ID（UUID，用来追踪、排查、去重）
	Event     string    `json:"event"`             // 事件类型（order_created / order_paid / order_timeout 等）
	BizID     uint      `json:"biz_id"`            // 业务ID（订单号、商品ID、支付ID）
	UserID    uint      `json:"user_id"`           // 操作用户ID（谁触发的这条消息）
	CreatedAt time.Time `json:"created_at"`        // 消息创建时间
	Payload   any       `json:"payload,omitempty"` // 消息真正的业务数据（任意结构体）
}
