package dao

import (
	"mall/model"
)

type AddressDB interface {
	Create(address *model.UserAddress) error
	GetByID(userID uint, addressID uint) (*model.UserAddress, error) // GetByID 查询单条地址
	GetDefault(userID uint) (*model.UserAddress, error)               // 取默认地址（无默认则取最近一条）
	ListByUserID(userID uint) ([]model.UserAddress, error)
	CountByUserID(userID uint) (int64, error)
	Update(address *model.UserAddress) error
	Delete(userID uint, addressID uint) (int64, error)
	ClearDefault(userID uint) error
	SetDefault(userID uint, addressID uint) (int64, error)
}
