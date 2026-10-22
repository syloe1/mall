package dao

import (
	"mall/model"

	"gorm.io/gorm"
)

type GormCartDB struct {
	DB *gorm.DB
}

// 加入购物车前，判断是否已存在
func (g *GormCartDB) GetItem(userID uint, productID uint) (*model.CartItem, error) {
	var item model.CartItem
	/*
		select * from cart_items where user_id = ? and product_id = ? limit 1
	*/
	if err := g.DB.
		Where("user_id = ? AND product_id = ?", userID, productID).
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// 添加商品到购物车
func (g *GormCartDB) CreateItem(item *model.CartItem) error {
	//insert into cart_items (...)
	return g.DB.Create(item).Error
}
func (g *GormCartDB) UpdateItem(item *model.CartItem) error {
	/*
		update cart_items set quantity = ? , checked = ? where id = ? and userid=?
	*/
	return g.DB.Model(&model.CartItem{}).
		Where("id = ? AND user_id = ?", item.ID, item.UserID).
		Updates(map[string]any{
			"quantity": item.Quantity,
			"checked":  item.Checked,
		}).Error
}

// 删除购物车中某个商品
func (g *GormCartDB) DeleteItem(userID uint, productID uint) (int64, error) {
	//delete from cart_items where user_id = ? and product_id=?
	result := g.DB.
		Where("user_id = ? AND product_id = ?", userID, productID).
		Delete(&model.CartItem{})

	return result.RowsAffected, result.Error
}

func (g *GormCartDB) Clear(userID uint) error {
	//delete from cart_items where user_id = ?
	return g.DB.
		Where("user_id = ?", userID).
		Delete(&model.CartItem{}).Error
}
func (g *GormCartDB) ListItems(userID uint) ([]model.CartItem, error) {
	var items []model.CartItem
	err := g.DB.
		Where("user_id = ?", userID).
		Preload("Product").
		Preload("Product.Images", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC")
		}).
		Order("created_at DESC").
		Find(&items).Error

	return items, err
}

func (g *GormCartDB) ListCheckedItems(userID uint) ([]model.CartItem, error) {
	var items []model.CartItem
	//SELECT * FROM cart_items WHERE user_id=? AND checked=1;
	err := g.DB.
		Where("user_id = ? AND checked = ?", userID, true).
		Preload("Product").
		Preload("Product.Images", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC")
		}).
		Order("created_at DESC").
		Find(&items).Error

	return items, err
}

func (g *GormCartDB) SetItemChecked(userID uint, productID uint, checked bool) (int64, error) {
	//UPDATE cart_items SET checked=? WHERE user_id=? AND product_id=?;
	result := g.DB.
		Model(&model.CartItem{}).
		Where("user_id = ? AND product_id = ?", userID, productID).
		Update("checked", checked)

	return result.RowsAffected, result.Error
}

func (g *GormCartDB) SetAllChecked(userID uint, checked bool) error {
	return g.DB.
		Model(&model.CartItem{}).
		Where("user_id = ?", userID).
		Update("checked", checked).Error
}
