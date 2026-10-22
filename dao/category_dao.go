package dao

import (
	"mall/model"

	"gorm.io/gorm"
)

type CategoryDB interface {
	List() ([]model.Category, error)
}

type GormCategoryDB struct {
	DB *gorm.DB
}

func (g *GormCategoryDB) List() ([]model.Category, error) {
	var categories []model.Category
	if err := g.DB.Order("sort_order ASC, id ASC").Find(&categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}
