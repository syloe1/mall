package dto

// binding是gin框架的参数校验标签
type FavoriteFolderCreateReq struct {
	Name string `json:"name" binding:"required"`
}
type FavoriteFolderUpdateReq struct {
	ID   uint   `json:"id" binding:"required"`
	Name string `json:"name" binding:"required"`
}
type FavoriteFolderDeleteReq struct {
	ID uint `json:"id" binding:"required"`
}
type FavoriteItemAddReq struct {
	FolderID  uint `json:"folder_id" binding:"required"`
	ProductID uint `json:"product_id" binding:"required"`
}
type FavoriteItemDeleteReq struct {
	FolderID  uint `json:"folder_id" binding:"required"`
	ProductID uint `json:"product_id" binding:"required"`
}

// 获取某个文件夹里的收藏商品列表
type FavoriteItemListReq struct {
	FolderID uint `json:"folder_id" binding:"required"`
	Page     int  `json:"page" binding:"min=0"`
	PageSize int  `json:"page_size" binding:"min=0"`
}
type FavoriteCheckReq struct {
	ProductID uint `json:"product_id" binding:"required"`
}
