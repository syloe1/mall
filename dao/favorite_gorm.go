package dao

import (
	"mall/model"

	"gorm.io/gorm"
)

type GormFavoriteDB struct {
	DB *gorm.DB
}

func (g *GormFavoriteDB) CreateFolder(folder *model.FavoriteFolder) error {
	return g.DB.Create(folder).Error
}
func (g *GormFavoriteDB) GetFolderByID(userID uint, folderID uint) (*model.FavoriteFolder, error) {
	var folder model.FavoriteFolder
	/*
		select * from favorite_folders
		where id = ? and user_id = ?
		LIMIT 1
	*/
	if err := g.DB.Where("id = ?  AND user_id = ?", folderID, userID).
		First(&folder).Error; err != nil {
		return nil, err
	}
	return &folder, nil
}
func (g *GormFavoriteDB) ListFolders(userID uint) ([]*model.FavoriteFolder, error) {
	var folders []*model.FavoriteFolder
	//默认文件夹排最前面, created_at升序
	/*
		select * from favorite_folders
		where user_id = ?
		order by is_default desc, created_at asc
	*/
	if err := g.DB.Where("user_id = ?", userID).
		Order("is_default DESC, created_at ASC").
		Find(&folders).Error; err != nil {
		return nil, err
	}
	return folders, nil
}
func (g *GormFavoriteDB) UpdateFolder(folder *model.FavoriteFolder) error {
	/*
		update favorite_folder s
		set name = ?, updated_at = ?
		where id = ? and user_id = ?
	*/
	return g.DB.
		Model(&model.FavoriteFolder{}).
		Where("id = ? AND user_id = ?", folder.ID, folder.UserID).
		Updates(map[string]any{
			"name": folder.Name,
		}).Error
}
func (g *GormFavoriteDB) DeleteFolder(userID uint, folderID uint) error {
	/*
		BEGIN

		DELETE FROM favorite_items
		where user_id = ? and folder_id = ?


		DELETE FROM favorite_folders
		where id = ? and user_id = ?

		COMMIT
	*/
	return g.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Where("user_id = ? AND folder_id = ?", userID, folderID).
			Delete(&model.FavoriteItem{}).Error; err != nil {
			return err
		}
		return tx.
			Where("id = ? AND user_id = ?", folderID, userID).
			Delete(&model.FavoriteFolder{}).Error
	})
}

func (g *GormFavoriteDB) AddItem(item *model.FavoriteItem) error {
	return g.DB.Create(item).Error
}
func (g *GormFavoriteDB) DeleteItem(userID uint, folderID uint, productID uint) (int64, error) {
	/*
		DELETE FROM favorite_items
		where user_id = ? AND folder_id = ? and product_id = ?
	*/
	result := g.DB.
		Where("user_id = ? AND folder_id = ? AND product_id = ?", userID, folderID, productID).
		Delete(&model.FavoriteItem{})
	return result.RowsAffected, result.Error
}
func (g *GormFavoriteDB) GetItem(userID uint, folderID uint, productID uint) (*model.FavoriteItem, error) {
	var item model.FavoriteItem
	/*
		select * from favorite_items
		where user_id = ? and folder_id = ? and product_id = ?
		limit 1
	*/
	if err := g.DB.
		Where("user_id = ? AND folder_id = ? AND product_id = ?", userID, folderID, productID).
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}
func (g *GormFavoriteDB) ListItems(query FavoriteItemQuery) ([]*model.FavoriteItem, int64, error) {
	var items []*model.FavoriteItem
	var total int64

	base := g.DB.
		Model(&model.FavoriteItem{}).
		Where("user_id = ? AND folder_id = ?", query.UserID, query.FolderID)

	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (query.Page - 1) * query.PageSize
	err := base.Session(&gorm.Session{}).
		Preload("Product").
		Preload("Product.Images", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC")
		}).
		Order("created_at DESC").
		Offset(offset).
		Limit(query.PageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
func (g *GormFavoriteDB) IsProductFavorited(userID uint, productID uint) (bool, error) {
	var count int64
	/*
		select count(*) from favorite_items
		where user_id = ? and product_id = ?
	*/
	err := g.DB.
		Model(&model.FavoriteItem{}).
		Where("user_id = ? AND product_id = ?", userID, productID).
		Count(&count).Error
	return count > 0, err
}
