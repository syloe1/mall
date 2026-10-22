package dao

import (
	"mall/model"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GormPaymentDB struct {
	DB *gorm.DB
}

func (g *GormPaymentDB) Create(payment *model.Payment) error {
	//INSERT INTO payments (...) VALUES (...);
	return g.DB.Create(payment).Error
}
func (g *GormPaymentDB) GetByPaymentNo(userID uint, paymentNo string) (*model.Payment, error) {
	//prelaod两条SQL Join 1条
	/*
		select * from payment where user_id = ? and payment_no = ? limit 1

		select * from orders where id = ?
	*/
	var payment model.Payment
	err := g.DB.Preload("Order").
		Where("user_id = ? AND payment_no = ?", userID, paymentNo).
		First(&payment).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

// 根据订单 ID 查支付单
func (g *GormPaymentDB) GetByOrderID(userID uint, orderID uint) (*model.Payment, error) {
	var payment model.Payment
	/*
		select * from payments where user_id = ? and order_id = ?
		order by created_at desc limit 1
	*/
	err := g.DB.
		Preload("Order").
		Where("user_id = ? AND order_id = ?", userID, orderID).
		Order("created_at DESC").
		First(&payment).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

func (g *GormPaymentDB) GetByOrderIDForUpdate(userID uint, orderID uint) (*model.Payment, error) {
	var payment model.Payment
	/*
		select * from payments where user_id = ? AND order_id = ?
		order by created_at desc limit 1 for update
	*/
	err := g.DB.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Order").
		Where("user_id = ? AND order_id = ?", userID, orderID).
		Order("created_at DESC").
		First(&payment).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

func (g *GormPaymentDB) GetSuccessByOrderID(userID uint, orderID uint) (*model.Payment, error) {
	var payment model.Payment
	/*
		select * from payments where user_id = ? and order_id = ?
		and status = 'success' limit 1
	*/
	err := g.DB.
		Preload("Order").
		Where(
			"user_id = ? AND order_id = ? AND status = ?",
			userID,
			orderID,
			model.PaymentStatusSuccess,
		).
		First(&payment).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

// 分页查询我的支付记录
func (g *GormPaymentDB) ListByUser(query PaymentListQuery) ([]model.Payment, int64, error) {
	var payments []model.Payment
	var total int64

	base := g.DB.Model(&model.Payment{}).
		Where("user_id = ?", query.UserID)

	status := strings.TrimSpace(query.Status)
	if status != "" {
		base = base.Where("status = ?", status)
	}

	channel := strings.TrimSpace(query.Channel)
	if channel != "" {
		base = base.Where("channel = ?", channel)
	}

	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (query.Page - 1) * query.PageSize
	err := base.Session(&gorm.Session{}).
		Preload("Order").
		Order("created_at DESC").
		Offset(offset).
		Limit(query.PageSize).
		Find(&payments).Error
	if err != nil {
		return nil, 0, err
	}

	return payments, total, nil
}
func (g *GormPaymentDB) Update(payment *model.Payment) error {
	//update payments set ... where id = ?
	return g.DB.Save(payment).Error
}
