package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	PaymentStatusPending = "pending"
	PaymentStatusSuccess = "success"
	PaymentStatusFailed  = "failed"
	PaymentStatusClosed  = "closed"
)

const (
	PaymentChannelMock    = "mock"    // 模拟支付
	PaymentChannelBalance = "balance" // 余额支付
	PaymentChannelWechat  = "wechat"  // 微信支付
	PaymentChannelAlipay  = "alipay"  // 支付宝
)

type Payment struct {
	gorm.Model

	PaymentNo string `json:"payment_no" gorm:"size:64;uniqueIndex;not null"`

	OrderID uint   `json:"order_id" gorm:"uniqueIndex;not null"`
	OrderNo string `json:"order_no" gorm:"size:64;index;not null"`
	UserID  uint   `json:"user_id" gorm:"index;not null"`

	Channel string `json:"channel" gorm:"size:32;index;not null"`
	Amount  int64  `json:"amount" gorm:"not null"`
	Status  string `json:"status" gorm:"size:32;index;not null"`

	TransactionNo string     `json:"transaction_no" gorm:"size:128;index"`
	PaidAt        *time.Time `json:"paid_at"`

	Order Order `json:"order,omitempty" gorm:"foreignKey:OrderID"`
}
