package dao

import "mall/model"

type OrderListQuery struct {
	UserID   uint   // 自己的ID
	Status   string // 订单状态：待支付 / 已支付 / 已取消 / 已完成
	Page     int    // 分页
	PageSize int    // 每页数量
}
type AdminOrderListQuery struct {
	UserID   uint
	Status   string
	OrderNo  string // 可以按订单号搜索
	Page     int
	PageSize int
}

type OrderDB interface {
	//创建订单
	Create(order *model.Order) error
	//用户查询自己的某个订单
	GetByID(userID uint, orderNo uint) (*model.Order, error)
	//查询订单 + 支付、取消订单时用，防止并发错乱
	GetByIDForUpdate(userID uint, orderNo uint) (*model.Order, error)
	//管理员查询任意订单
	GetAdminByID(orderID uint) (*model.Order, error)
	//用户查询自己的订单列表（分页）
	ListByUser(query OrderListQuery) ([]*model.Order, int64, error)
	//管理员查询所有订单（分页 + 搜索）
	ListAdmin(query AdminOrderListQuery) ([]*model.Order, int64, error)
	//更新订单状态
	//待支付 → 已支付 → 已发货 → 已完成
	UpdateStatus(orderID uint, status string) error
	//用户取消订单
	Cancel(userID uint, orderNo uint) (int64, error)
	//扣库存 	创建订单 	清空购物车
	Transaction(fn func(txOrderDB OrderDB, txProductDB ProductDB, txCartDB CartDB) error) error
}
