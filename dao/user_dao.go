package dao

import (
	"mall/model"
)

/*
上层只依赖接口
下层负责实现
*/
type UserDB interface {
	Create(user *model.User) error                       // 创建用户
	FindById(id uint) (*model.User, error)              // 按 ID 查询
	FindByIDForUpdate(id uint) (*model.User, error)     // 加锁查询
	GetByUsername(username string) (*model.User, error) // 按用户名查
	Update(user *model.User) error                      // 更新用户
	Transaction(fn func(tx UserDB) error) error         // 事务
}

type JWTConfig interface {
	GetSecret() []byte
}
