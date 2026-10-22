package dto

// 添加商品到购物车
type CartItemAddReq struct {
	ProductID uint `json:"product_id" binding:"required"`
	Quantity  int  `json:"quantity" binding:"required,min=1"`
}

// 更新购物车商品数量
type CartItemUpdateReq struct {
	ProductID uint `json:"product_id" binding:"required"`
	Quantity  int  `json:"quantity" binding:"required,min=1"`
}

// 删除购物车商品
type CartItemDeleteReq struct {
	ProductID uint `json:"product_id" binding:"required"`
}

// 单个商品勾选 / 取消勾选
type CartItemCheckedReq struct {
	ProductID uint `json:"product_id" binding:"required"`
	Checked   bool `json:"checked"`
}

// 全选 / 取消全选
type CartCheckedAllReq struct {
	Checked bool `json:"checked"`
}
