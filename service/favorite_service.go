package service

import (
	"errors"
	"mall/dao"
	"mall/model"
	"mall/pkg/core"
	"net/http"
	"strings"

	"gorm.io/gorm"
)

type FavoriteService struct {
	db        dao.FavoriteDB
	productDB dao.ProductDB
}

type CreateFavoriteFolderInput struct {
	UserID uint
	Name   string
}

type UpdateFavoriteFolderInput struct {
	UserID   uint
	FolderID uint
	Name     string
}

type DeleteFavoriteFolderInput struct {
	UserID   uint
	FolderID uint
}

type AddFavoriteItemInput struct {
	UserID    uint
	FolderID  uint
	ProductID uint
}

type DeleteFavoriteItemInput struct {
	UserID    uint
	FolderID  uint
	ProductID uint
}

type ListFavoriteItemsInput struct {
	UserID   uint // 哪个用户
	FolderID uint // 哪个收藏夹
	Page     int  // 第几页
	PageSize int  // 每页几条
}
type ListFavoriteItemsResult struct {
	Items    []*model.FavoriteItem `json:"items"`
	Total    int64                 `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
}

func NewFavoriteService(db dao.FavoriteDB, productDB dao.ProductDB) *FavoriteService {
	return &FavoriteService{
		db:        db,
		productDB: productDB,
	}
}

func (s *FavoriteService) CreateFolder(input CreateFavoriteFolderInput) error {
	if input.UserID == 0 {
		return core.NewBizError(http.StatusBadRequest, "用户 ID 不合法")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return core.NewBizError(http.StatusBadRequest, "请填写收藏夹名称")
	}

	return s.db.CreateFolder(&model.FavoriteFolder{
		UserID:    input.UserID,
		Name:      name,
		IsDefault: false,
	})
}

func (s *FavoriteService) ListFolders(userID uint) ([]*model.FavoriteFolder, error) {
	if userID == 0 {
		return nil, core.NewBizError(http.StatusBadRequest, "用户 ID 不合法")
	}
	folders, err := s.db.ListFolders(userID)
	if err != nil {
		return nil, err
	}
	// 新用户首次访问：自动 provision 一个默认收藏夹（幂等）
	if len(folders) == 0 {
		if err := s.db.CreateFolder(&model.FavoriteFolder{
			UserID:    userID,
			Name:      "默认收藏夹",
			IsDefault: true,
		}); err != nil {
			return nil, err
		}
		return s.db.ListFolders(userID)
	}
	return folders, nil
}

func (s *FavoriteService) UpdateFolder(input UpdateFavoriteFolderInput) error {
	if input.UserID == 0 {
		return core.NewBizError(http.StatusBadRequest, "用户 ID 不合法")
	}
	if input.FolderID == 0 {
		return core.NewBizError(http.StatusBadRequest, "收藏夹 ID 不合法")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return core.NewBizError(http.StatusBadRequest, "请填写收藏夹名称")
	}

	folder, err := s.db.GetFolderByID(input.UserID, input.FolderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return core.NewBizError(http.StatusNotFound, "收藏夹不存在")
		}
		return err
	}

	folder.Name = name
	return s.db.UpdateFolder(folder)
}

func (s *FavoriteService) DeleteFolder(input DeleteFavoriteFolderInput) error {
	if input.UserID == 0 {
		return core.NewBizError(http.StatusBadRequest, "用户 ID 不合法")
	}
	if input.FolderID == 0 {
		return core.NewBizError(http.StatusBadRequest, "收藏夹 ID 不合法")
	}

	folder, err := s.db.GetFolderByID(input.UserID, input.FolderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return core.NewBizError(http.StatusNotFound, "收藏夹不存在")
		}
		return err
	}
	if folder.IsDefault {
		return core.NewBizError(http.StatusBadRequest, "默认收藏夹不能删除")
	}

	return s.db.DeleteFolder(input.UserID, input.FolderID)
}

func (s *FavoriteService) AddItem(input AddFavoriteItemInput) error {
	if input.UserID == 0 {
		return core.NewBizError(http.StatusBadRequest, "用户 ID 不合法")
	}
	if input.FolderID == 0 {
		return core.NewBizError(http.StatusBadRequest, "收藏夹 ID 不合法")
	}
	if input.ProductID == 0 {
		return core.NewBizError(http.StatusBadRequest, "商品 ID 不合法")
	}
	//收藏夹存在吗
	if _, err := s.db.GetFolderByID(input.UserID, input.FolderID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return core.NewBizError(http.StatusNotFound, "收藏夹不存在")
		}
		return err
	}
	//商品存在吗
	if _, err := s.productDB.GetProductById(input.ProductID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return core.NewBizError(http.StatusNotFound, "商品不存在或已下架")
		}
		return err
	}
	//是否收藏过
	if _, err := s.db.GetItem(input.UserID, input.FolderID, input.ProductID); err == nil {
		return core.NewBizError(http.StatusBadRequest, "该商品已收藏")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	return s.db.AddItem(&model.FavoriteItem{
		UserID:    input.UserID,
		FolderID:  input.FolderID,
		ProductID: input.ProductID,
	})
}

func (s *FavoriteService) DeleteItem(input DeleteFavoriteItemInput) error {
	if input.UserID == 0 {
		return core.NewBizError(http.StatusBadRequest, "用户 ID 不合法")
	}
	if input.FolderID == 0 {
		return core.NewBizError(http.StatusBadRequest, "收藏夹 ID 不合法")
	}
	if input.ProductID == 0 {
		return core.NewBizError(http.StatusBadRequest, "商品 ID 不合法")
	}

	rowsAffected, err := s.db.DeleteItem(input.UserID, input.FolderID, input.ProductID)
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return core.NewBizError(http.StatusNotFound, "收藏记录不存在")
	}

	return nil
}

func (s *FavoriteService) ListItems(input ListFavoriteItemsInput) (*ListFavoriteItemsResult, error) {
	if input.UserID == 0 {
		return nil, core.NewBizError(http.StatusBadRequest, "用户 ID 不合法")
	}
	if input.FolderID == 0 {
		return nil, core.NewBizError(http.StatusBadRequest, "收藏夹 ID 不合法")
	}
	if input.Page <= 0 {
		input.Page = 1
	}
	//分页参数安全处理
	if input.PageSize <= 0 {
		input.PageSize = 20
	}
	if input.PageSize > 100 {
		input.PageSize = 100
	}
	//收藏夹存在吗
	if _, err := s.db.GetFolderByID(input.UserID, input.FolderID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.NewBizError(http.StatusNotFound, "收藏夹不存在")
		}
		return nil, err
	}

	items, total, err := s.db.ListItems(dao.FavoriteItemQuery{
		UserID:   input.UserID,
		FolderID: input.FolderID,
		Page:     input.Page,
		PageSize: input.PageSize,
	})
	if err != nil {
		return nil, err
	}

	return &ListFavoriteItemsResult{
		Items:    items,
		Total:    total,
		Page:     input.Page,
		PageSize: input.PageSize,
	}, nil
}

func (s *FavoriteService) IsProductFavorited(userID uint, productID uint) (bool, error) {
	if userID == 0 {
		return false, core.NewBizError(http.StatusBadRequest, "用户 ID 不合法")
	}
	if productID == 0 {
		return false, core.NewBizError(http.StatusBadRequest, "商品 ID 不合法")
	}

	return s.db.IsProductFavorited(userID, productID)
}
