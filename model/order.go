package model

import "gorm.io/gorm"

const (
	OrderPendingPay = "pending_pay" //待支付
	OrderPaid       = "paid"
	OrderShipped    = "shipped" //已发货
	OrderCompleted  = "completed"
	OrderCanceled   = "canceled"
)

type Order struct {
	gorm.Model
	OrderNo     string `json:"order_no" gorm:"size:64;uniqueIndex;not null"`
	UserID      uint   `json:"user_id" gorm:"index;not null"`
	Status      string `json:"status" gorm:"size:32;index;not null"`
	TotalAmount int64  `json:"total_amount" gorm:"not null"`

	ReceiverName string `json:"receiver_name" gorm:"size:64;not null"`
	Phone        string `json:"phone" gorm:"size:32;not null"`
	Province     string `json:"province" gorm:"size:64;not null"`
	City         string `json:"city" gorm:"size:64;not null"`
	District     string `json:"district" gorm:"size:64;not null"`
	Detail       string `json:"detail" gorm:"size:255;not null"`
	PostalCode   string `json:"postal_code" gorm:"size:16"`

	Items []OrderItem `json:"items" gorm:"foreignKey:OrderID"`
}

type OrderItem struct {
	gorm.Model
	OrderID      uint   `json:"order_id" gorm:"index;not null"` //属于哪个订单
	ProductID    uint   `json:"product_id" gorm:"index;not null"`
	ProductName  string `json:"product_name" gorm:"size:128;not null"` //下单时的商品名称快照
	ProductImage string `json:"product_image" gorm:"size:255"`
	PriceCents   int64  `json:"price_cents" gorm:"not null"` //下单时价格快照
	Quantity     int    `json:"quantity" gorm:"not null"`
	Subtotal     int64  `json:"subtotal" gorm:"not null"`
}
