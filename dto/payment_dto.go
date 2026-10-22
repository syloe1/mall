package dto

// PaymentCreateReq 发起支付
// 发起支付 / 提交支付
type PaymentCreateReq struct {
	OrderID     uint   `json:"order_id" binding:"required"`
	Channel     string `json:"channel" binding:"required"`
	PayPassword string `json:"pay_password"`
}

// PaymentGetReq 查询支付单状态
type PaymentGetReq struct {
	PaymentNo string `json:"payment_no" binding:"required"`
}

// PaymentMockSuccessReq 模拟支付成功
type PaymentMockSuccessReq struct {
	PaymentNo string `json:"payment_no" binding:"required"`
}
