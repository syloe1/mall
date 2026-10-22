package dao

import (
	"mall/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GormUserDB struct {
	DB *gorm.DB
}

func (g *GormUserDB) Create(user *model.User) error {
	return g.DB.Create(user).Error
}
func (g *GormUserDB) FindById(id uint) (*model.User, error) {
	var user model.User
	//select * from users where id = id LIMIT 1
	if err := g.DB.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// 查询并加行锁
func (g *GormUserDB) FindByIDForUpdate(id uint) (*model.User, error) {
	var user model.User
	//select * from users where id = ? LIMIT 1 FOR UPDATE
	//for update行锁， 悲观锁
	if err := g.DB.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// 根据用户名查用户（登录用）
func (g *GormUserDB) GetByUsername(username string) (*model.User, error) {
	var user model.User
	//select * from users where username = ? LIMIT 1;
	if err := g.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
// Update 更新用户。
func (g *GormUserDB) Update(user *model.User) error {
	return g.DB.Save(user).Error
}

func (g *GormUserDB) Transaction(fn func(tx UserDB) error) error {
	/*
		BEGIN;
		UPDATE users;
		INSERT INTO orders;
		COMMIT  |  ROLLBACK
	*/
	return g.DB.Transaction(func(tx *gorm.DB) error {
		return fn(&GormUserDB{DB: tx})
	})
}
