package dto

// 后台创建商品
type ProductCreateReq struct {
	Name        string   `json:"name" binding:"required"`
	PriceCents  int64    `json:"price_cents" binding:"required,min=1"`
	Description string   `json:"description"`
	Stock       int      `json:"stock" binding:"min=0"`
	CategoryID  uint     `json:"category_id" binding:"required"`
	Status      string   `json:"status"`
	Images      []string `json:"images"`
}

// 后台更新商品
type ProductUpdateReq struct {
	ID          uint     `json:"id" binding:"required"`
	Name        string   `json:"name" binding:"required"`
	PriceCents  int64    `json:"price_cents" binding:"required,min=1"`
	Description string   `json:"description"`
	Stock       int      `json:"stock" binding:"min=0"`
	CategoryID  uint     `json:"category_id" binding:"required"`
	Status      string   `json:"status"`
	Images      []string `json:"images"`
}

// 删除商品
type ProductDeleteReq struct {
	ID uint `json:"id" binding:"required"`
}

// 后台商品管理列表（GET，使用 query 参数，浏览器可直接传参）
type ProductListAdminReq struct {
	Page       int    `form:"page" json:"page"`
	PageSize   int    `form:"page_size" json:"page_size"`
	Status     string `form:"status" json:"status"`
	CategoryID uint   `form:"category_id" json:"category_id"`
	Keyword    string `form:"keyword" json:"keyword"`
}

// 用户端搜索商品 url查询
type ProductSearchReq struct {
	Page       int    `form:"page" binding:"min=0"`
	PageSize   int    `form:"page_size" binding:"min=0"`
	Keyword    string `form:"keyword"`
	CategoryID uint   `form:"category_id"`
}
