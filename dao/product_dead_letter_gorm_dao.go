package dao

import (
	"mall/model"

	"gorm.io/gorm"
)

type GormProductDeadLetterDB struct {
	DB *gorm.DB
}

func (g *GormProductDeadLetterDB) Create(record *model.ProductEventDeadLetter) error {
	return g.DB.Create(record).Error
}

func (g *GormProductDeadLetterDB) List(query ProductDeadLetterQuery) ([]model.ProductEventDeadLetter, int64, error) {
	/*
		-- 统计总数
		SELECT COUNT(*) FROM product_event_dead_letters
		WHERE source = ? AND event_type = ? AND product_id = ?;

		-- 分页查询
		SELECT * FROM product_event_dead_letters
		WHERE ...
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?;
	*/
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}

	db := g.DB.Model(&model.ProductEventDeadLetter{})
	if query.Source != "" {
		db = db.Where("source = ?", query.Source)
	}
	if query.EventType != "" {
		db = db.Where("event_type = ?", query.EventType)
	}
	if query.ProductID > 0 {
		db = db.Where("product_id = ?", query.ProductID)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []model.ProductEventDeadLetter
	offset := (query.Page - 1) * query.PageSize
	if err := db.Order("created_at DESC").Offset(offset).Limit(query.PageSize).Find(&records).Error; err != nil {
		return nil, 0, err
	}

	return records, total, nil
}
