package dao

import (
	"mall/model"
)

type CartDB interface {
	//查询某个用户、某个商品是否已经在购物车里
	GetItem(userID uint, productID uint) (*model.CartItem, error)
	//添加商品到购物车
	CreateItem(item *model.CartItem) error
	//更新购物车商品数量
	UpdateItem(item *model.CartItem) error
	//删除购物车中的某个商品
	DeleteItem(userID uint, productID uint) (int64, error)
	Clear(userID uint) error
	//查询购物车所有商品
	ListItems(userID uint) ([]model.CartItem, error)
	//只查询【勾选】的商品
	ListCheckedItems(userID uint) ([]model.CartItem, error)
	//勾选 / 取消勾选 单个商品s
	SetItemChecked(userID uint, productID uint, checked bool) (int64, error)
	//全选 / 取消全选 购物车
	SetAllChecked(userID uint, checked bool) error
}
