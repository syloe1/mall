package database

import (
	"mall/model"

	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{},
		&model.Product{},
		&model.ProductImage{},
		&model.ProductOutbox{},
		&model.ProductEventDeadLetter{},
		&model.FavoriteFolder{},
		&model.FavoriteItem{},
		&model.UserAddress{},
		&model.CartItem{},
		&model.Order{},
		&model.OrderItem{},
		&model.Payment{},
		&model.SeckillActivity{},
		&model.SeckillPurchase{},
		&model.Category{},
	)
}
