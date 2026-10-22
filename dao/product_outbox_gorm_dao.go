package dao

import (
	"mall/model"
	"time"

	"gorm.io/gorm"
)

type GormProductOutboxDB struct {
	DB *gorm.DB
}

func (g *GormProductOutboxDB) Create(event *model.ProductOutbox) error {
	//INSERT INTO product_outboxes () VALUE ()
	return g.DB.Create(event).Error
}

func (g *GormProductOutboxDB) ListPending(limit int) ([]model.ProductOutbox, error) {
	if limit <= 0 {
		limit = 50
	}
	/*
		select * from product_outboxes
		where status = "pending"
		order by created_at asc
		limit 50
	*/
	var events []model.ProductOutbox
	err := g.DB.
		Where("status = ?", model.ProductOutboxPending).
		Order("created_at ASC").
		Limit(limit).
		Find(&events).Error
	return events, err
}

func (g *GormProductOutboxDB) MarkPublished(id uint) error {
	now := time.Now()
	/*
		UPDATE product_outboxes
		set status = 'published' , published_at = ?,last_error = ''
		where id = ?
	*/
	return g.DB.Model(&model.ProductOutbox{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":       model.ProductOutboxPublished,
			"published_at": &now,
			"last_error":   "",
		}).Error
}

func (g *GormProductOutboxDB) MarkFailed(id uint, errMsg string) error {
	/*
		Update product_outboxes
		set status = 'failed', retry_count = retry_count + 1, last_error = ?
		where id = ?
	*/
	return g.DB.Model(&model.ProductOutbox{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":      model.ProductOutboxFailed,
			"retry_count": gorm.Expr("retry_count + ?", 1),
			"last_error":  errMsg,
		}).Error
}
