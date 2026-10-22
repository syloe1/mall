package model

import "gorm.io/gorm"

type CartItem struct {
	gorm.Model
	UserID    uint `json:"user_id" gorm:"index;not null"`
	ProductID uint `json:"product_id" gorm:"index;not null"`
	Quantity  int  `json:"quantity" gorm:"not null;default:1"`
	//是否勾选，后面下单只处理 checked=true 的商品
	Checked bool `json:"checked" gorm:"not null;default:true"`
	//查询购物车时可以 Preload("Product") 带出商品信息
	Product Product `json:"product,omitempty" gorm:"foreignKey:ProductID"`
}
