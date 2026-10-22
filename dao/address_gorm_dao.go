package dao

import (
	"mall/model"

	"gorm.io/gorm"
)

type GormAddressDB struct {
	DB *gorm.DB
}

func (g *GormAddressDB) Create(address *model.UserAddress) error {
	//INSERT INTO user_addresses (...) VALUES (...);
	return g.DB.Create(address).Error
}
func (g *GormAddressDB) GetByID(userID uint, addressID uint) (*model.UserAddress, error) {
	var address model.UserAddress
	//SELECT * FROM user_addresses
	//WHERE id = ? AND user_id = ? LIMIT 1;
	if err := g.DB.Where("id = ? AND user_id = ?", addressID, userID).
		First(&address).Error; err != nil {
		return nil, err
	}
	return &address, nil
}

// GetDefault 取默认地址；没有标记默认的就取最近一条
func (g *GormAddressDB) GetDefault(userID uint) (*model.UserAddress, error) {
	var address model.UserAddress
	err := g.DB.Where("user_id = ?", userID).
		Order("is_default DESC, id DESC").
		First(&address).Error
	if err != nil {
		return nil, err
	}
	return &address, nil
}

func (g *GormAddressDB) ListByUserID(userID uint) ([]model.UserAddress, error) {
	var addresses []model.UserAddress
	/*
		SELECT * FROM user_addresses
		WHERE user_id = ?
		ORDER BY is_default DESC, created_at DESC;
	*/
	err := g.DB.Where("user_id = ?", userID).
		Order("is_default DESC,created_at DESC").
		Find(&addresses).Error
	return addresses, err
}
func (g *GormAddressDB) CountByUserID(userID uint) (int64, error) {
	var count int64
	//SELECT COUNT(*) FROM user_addresses WHERE user_id = ?
	err := g.DB.Model(&model.UserAddress{}).
		Where("user_id = ?", userID).
		Count(&count).Error
	return count, err
}
func (g *GormAddressDB) Update(address *model.UserAddress) error {
	/*
		UPDATE user_addresses
		SET receiver_name=?, phone=?, ...
		WHERE id=? AND user_id=?
	*/
	return g.DB.Model(&model.UserAddress{}).
		Where("id = ? AND user_id = ?", address.ID, address.UserID).
		Updates(map[string]any{
			"receiver_name": address.ReceiverName,
			"phone":         address.Phone,
			"province":      address.Province,
			"city":          address.City,
			"district":      address.District,
			"detail":        address.Detail,
			"postal_code":   address.PostalCode,
			"is_default":    address.IsDefault,
		}).Error
}
func (g *GormAddressDB) ClearDefault(userID uint) error {
	/*
		UPDATE user_addresses
		SET is_default = false
		WHERE user_id = ?
	*/
	return g.DB.
		Model(&model.UserAddress{}).
		Where("user_id = ?", userID).
		Update("is_default", false).Error
}

func (g *GormAddressDB) SetDefault(userID uint, addressID uint) (int64, error) {
	/*
		UPDATE user_addresses
		SET is_default = true
		WHERE id = ? AND user_id = ?
	*/
	result := g.DB.
		Model(&model.UserAddress{}).
		Where("id = ? AND user_id = ?", addressID, userID).
		Update("is_default", true)

	return result.RowsAffected, result.Error
}
func (g *GormAddressDB) Delete(userID uint, addressID uint) (int64, error) {
	/*
		DELETE FROM user_addresses
		WHERE id = ? AND user_id = ?
	*/
	result := g.DB.
		Where("id = ? AND user_id = ?", addressID, userID).
		Delete(&model.UserAddress{})

	return result.RowsAffected, result.Error
}
