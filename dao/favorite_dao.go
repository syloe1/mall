package dao

import (
	"mall/model"
)

type FavoriteItemQuery struct {
	UserID   uint // 用户
	FolderID uint // 文件夹ID
	Page     int  // 分页
	PageSize int
}
type FavoriteDB interface {
	CreateFolder(folder *model.FavoriteFolder) error
	GetFolderByID(userID uint, folderID uint) (*model.FavoriteFolder, error)
	ListFolders(userID uint) ([]*model.FavoriteFolder, error)
	UpdateFolder(folder *model.FavoriteFolder) error
	DeleteFolder(userID uint, folderID uint) error

	//添加商品到收藏夹
	AddItem(item *model.FavoriteItem) error
	//取消收藏（从某个文件夹删除某个商品）
	DeleteItem(userID uint, folderID uint, productID uint) (int64, error)
	//查询某个商品是否已收藏
	GetItem(userID uint, folderID uint, productID uint) (*model.FavoriteItem, error)
	ListItems(query FavoriteItemQuery) ([]*model.FavoriteItem, int64, error)
	//检查商品是否被该用户收藏（商品详情页用）
	IsProductFavorited(userID uint, productID uint) (bool, error)
}
