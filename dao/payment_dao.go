package dao

import "mall/model"

type PaymentListQuery struct {
	UserID   uint   // 用户ID
	Status   string // 支付状态：待支付/成功/失败
	Channel  string // 支付渠道：支付宝/微信/余额
	Page     int    // 页码
	PageSize int    // 每页条数
}

type PaymentDB interface {
	Create(payment *model.Payment) error //创建支付单

	GetByPaymentNo(userID uint, paymentNo string) (*model.Payment, error)    //根据支付单号查询
	GetByOrderID(userID uint, orderID uint) (*model.Payment, error)          //. 根据订单 ID 查询支付单
	GetByOrderIDForUpdate(userID uint, orderID uint) (*model.Payment, error) //根据订单 ID 查询 + 加锁（FOR UPDATE）

	GetSuccessByOrderID(userID uint, orderID uint) (*model.Payment, error) //查询订单是否已支付成功
	ListByUser(query PaymentListQuery) ([]model.Payment, int64, error)     //分页查询用户的支付记录
	Update(payment *model.Payment) error
}
