package model

// Category 商品分类字典（替代前端硬编码映射）。
type Category struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Name      string `gorm:"size:32;not null" json:"name"`
	SortOrder int    `gorm:"not null;default:0" json:"sort_order"`
}
