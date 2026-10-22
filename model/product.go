package model

import (
	"gorm.io/gorm"
)

const (
	ProductDraft   = "draft"
	ProductOnSale  = "on_sale"
	ProductOffSale = "off_sale"
	ProductSoldOut = "sold_out"
)

type Product struct {
	gorm.Model
	Name        string         `json:"name"`
	PriceCents  int64          `json:"price_cents"`
	Description string         `json:"description"`
	Stock       int            `json:"stock"`
	CategoryID  uint           `json:"category_id"`
	Status      string         `json:"status"`
	SaleCount   int            `json:"sale_count"`
	Images      []ProductImage `json:"images" gorm:"foreignKey:ProductID"`
}
type ProductImage struct {
	gorm.Model
	ProductID uint   `json:"product_id" gorm:"index"`
	URL       string `json:"url"`
	SortOrder int    `json:"sort_order"`
}
