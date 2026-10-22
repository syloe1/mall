package model

import (
	"gorm.io/gorm"
)

type FavoriteFolder struct {
	gorm.Model
	//这个收藏夹属于哪个用户
	UserID    uint   `json:"user_id" gorm:"index;not null"`
	Name      string `json:"name" gorm:"size:64;not null"`
	IsDefault bool   `json:"is_default" gorm:"not null;default:false"`

	//这个收藏夹下面有哪些商品
	Items []FavoriteItem `json:"items,omitempty" gorm:"foreignKey:FolderID"`
}

type FavoriteItem struct {
	gorm.Model
	UserID    uint `json:"user_id" gorm:"index;not null"`
	FolderID  uint `json:"folder_id" gorm:"index;not null"`
	ProductID uint `json:"product_id" gorm:"index;not null"`

	Folder FavoriteFolder `json:"folder,omitempty" gorm:"foreignKey:FolderID"`
	//后面查询收藏夹商品时可以 Preload("Product")
	Product Product `json:"product,omitempty" gorm:"foreignKey:ProductID"`
}
