package service

import (
	"mall/dao"
	"mall/model"
)

type CategoryService struct {
	db dao.CategoryDB
}

func NewCategoryService(db dao.CategoryDB) *CategoryService {
	return &CategoryService{db: db}
}

func (s *CategoryService) List() ([]model.Category, error) {
	return s.db.List()
}
