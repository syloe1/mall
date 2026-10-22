package dto

// 商品分类（前端导航、筛选使用）
type CategoryResponse struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}
