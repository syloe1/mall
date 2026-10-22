package dto

// 从购物车勾选商品创建订单
type OrderCreateFromCartReq struct {
	AddressID uint `json:"address_id" binding:"required"`
}

// 直接购买某个商品创建订单
type OrderBuyNowReq struct {
	ProductID uint `json:"product_id" binding:"required"`
	Quantity  uint `json:"quantity" binding:"required,min=1"`
	AddressID uint `json:"address_id" binding:"required"`
}

// 获取单个订单详情
type OrderGetReq struct {
	ID uint `json:"id" binding:"required"`
}

// 用户查询自己的订单列表
type OrderListReq struct {
	Page     int    `json:"page" binding:"min=0"`
	PageSize int    `json:"page_size" binding:"min=0"`
	Status   string `json:"status"`
}

// 用户取消订单
type OrderCancelReq struct {
	ID uint `json:"id" binding:"required"`
}

// 管理员查询所有订单
type AdminOrderListReq struct {
	Page     int    `json:"page" binding:"min=0"`
	PageSize int    `json:"page_size" binding:"min=0"`
	Status   string `json:"status"`
	UserID   uint   `json:"user_id"`
	OrderNo  string `json:"order_no"`
}

// 管理员修改订单状态
type AdminOrderStatusUpdateReq struct {
	ID     uint   `json:"id" binding:"required"` //订单ID
	Status string `json:"status" binding:"required"`
}
