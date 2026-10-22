package dao

import (
	"mall/model"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GormOrderDB struct {
	DB *gorm.DB
}

func (g *GormOrderDB) Transaction(fn func(txOrderDB OrderDB, txProductDB ProductDB, txCartDB CartDB) error) error {
	/*
		BEGIN

		扣库存
		创建订单
		清空购物车

		COMMIT
	*/
	return g.DB.Transaction(func(tx *gorm.DB) error {
		return fn(
			&GormOrderDB{DB: tx},
			&GormProductDB{DB: tx},
			&GormCartDB{DB: tx},
		)
	})
}

func (g *GormOrderDB) Create(order *model.Order) error {
	//INSERT INTO ORDER () VALUES （）
	return g.DB.Create(order).Error
}
func (g *GormOrderDB) GetByID(userID uint, orderNo uint) (*model.Order, error) {
	var order model.Order
	//SELECT * FROM orders WHERE id = ？ AND user_id = ?
	//LIMIT 1
	if err := g.DB.Preload("Items").
		Where("id = ? AND user_id = ?", orderNo, userID).
		First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (g *GormOrderDB) GetByIDForUpdate(userID uint, orderNo uint) (*model.Order, error) {
	var order model.Order
	/*
		SELECT * FROM orders where id = ? and user_id = ?
		LIMIT 1 FOR update
	*/
	if err := g.DB.Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Items").
		Where("id = ? AND user_id = ?", orderNo, userID).
		First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (g *GormOrderDB) GetAdminByID(orderNo uint) (*model.Order, error) {
	var order model.Order
	/*
		SELECT * FROM orders where id = ? LIMIT 1
		SELECT * FROM order_items where order_id = ?
	*/
	if err := g.DB.Preload("Items").
		Where("id = ?", orderNo).
		First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (g *GormOrderDB) ListByUser(query OrderListQuery) ([]*model.Order, int64, error) {
	var orders []*model.Order
	var total int64

	base := g.DB.
		Model(&model.Order{}).
		Where("user_id = ?", query.UserID)

	status := strings.TrimSpace(query.Status)
	if status != "" {
		base = base.Where("status = ?", status)
	}

	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (query.Page - 1) * query.PageSize
	err := base.Session(&gorm.Session{}).
		Preload("Items").
		Order("created_at DESC").
		Offset(offset).
		Limit(query.PageSize).
		Find(&orders).Error
	if err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

func (g *GormOrderDB) ListAdmin(query AdminOrderListQuery) ([]*model.Order, int64, error) {
	var orders []*model.Order
	var total int64

	base := g.DB.Model(&model.Order{})

	if query.UserID > 0 {
		base = base.Where("user_id = ?", query.UserID)
	}

	status := strings.TrimSpace(query.Status)
	if status != "" {
		base = base.Where("status = ?", status)
	}

	orderNo := strings.TrimSpace(query.OrderNo)
	if orderNo != "" {
		base = base.Where("order_no LIKE ?", "%"+orderNo+"%")
	}

	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (query.Page - 1) * query.PageSize
	err := base.Session(&gorm.Session{}).
		Preload("Items").
		Order("created_at DESC").
		Offset(offset).
		Limit(query.PageSize).
		Find(&orders).Error
	if err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

func (g *GormOrderDB) UpdateStatus(orderID uint, status string) error {
	/*
		Update orders set status = ?, updated_at = ?
		where id = ?
	*/
	return g.DB.
		Model(&model.Order{}).
		Where("id = ?", orderID).
		Update("status", status).Error
}

func (g *GormOrderDB) Cancel(userID uint, orderID uint) (int64, error) {
	/*
		Update orders set status = 'canceled'
		where id = ? and user_id = ? and status = 'pending_pay'
	*/
	result := g.DB.
		Model(&model.Order{}).
		Where("id = ? AND user_id = ? AND status = ?", orderID, userID, model.OrderPendingPay).
		Update("status", model.OrderCanceled)

	return result.RowsAffected, result.Error
}
