package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"mall/dto"
	"mall/pkg/core"
	"mall/service"
)

type FavoriteHandler struct {
	svc *service.FavoriteService
}

func NewFavoriteHandler(svc *service.FavoriteService) *FavoriteHandler {
	return &FavoriteHandler{svc: svc}
}

// @Summary      创建收藏夹
// @Tags         收藏
// @Accept       json
// @Produce      json
// @Param        body  body  dto.FavoriteFolderCreateReq  true  "收藏夹名称"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/favorite/folders [post]
func (h *FavoriteHandler) CreateFolder(c *gin.Context) {
	var req dto.FavoriteFolderCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", err)
		return
	}
	if err := h.svc.CreateFolder(service.CreateFavoriteFolderInput{
		UserID: c.GetUint("userID"),
		Name:   req.Name,
	}); err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "create favorite folder success", nil)
}

// @Summary      收藏夹列表
// @Tags         收藏
// @Accept       json
// @Produce      json
// @Success      200  {object}  core.Response
// @Failure      401  {object}  core.Response
// @Security     BearerAuth
// @Router       /users/favorite/folders [get]
func (h *FavoriteHandler) ListFolders(c *gin.Context) {
	folders, err := h.svc.ListFolders(c.GetUint("userID"))
	if err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "list favorite folders success", folders)
}

// @Summary      更新收藏夹
// @Tags         收藏
// @Accept       json
// @Produce      json
// @Param        body  body  dto.FavoriteFolderUpdateReq  true  "收藏夹ID和新名称"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/favorite/folders [put]
func (h *FavoriteHandler) UpdateFolder(c *gin.Context) {
	var req dto.FavoriteFolderUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", err)
		return
	}
	if err := h.svc.UpdateFolder(service.UpdateFavoriteFolderInput{
		UserID:   c.GetUint("userID"),
		FolderID: req.ID,
		Name:     req.Name,
	}); err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "update favorite folder success", nil)
}

// @Summary      删除收藏夹
// @Tags         收藏
// @Accept       json
// @Produce      json
// @Param        body  body  dto.FavoriteFolderDeleteReq  true  "收藏夹ID"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/favorite/folders [delete]
func (h *FavoriteHandler) DeleteFolder(c *gin.Context) {
	var req dto.FavoriteFolderDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", err)
		return
	}
	if err := h.svc.DeleteFolder(service.DeleteFavoriteFolderInput{
		UserID:   c.GetUint("userID"),
		FolderID: req.ID,
	}); err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "delete favorite folder success", nil)
}

// @Summary      收藏商品
// @Description  将商品添加到指定收藏夹
// @Tags         收藏
// @Accept       json
// @Produce      json
// @Param        body  body  dto.FavoriteItemAddReq  true  "收藏夹ID和商品ID"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/favorite/items [post]
func (h *FavoriteHandler) AddItem(c *gin.Context) {
	var req dto.FavoriteItemAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", err)
		return
	}
	if err := h.svc.AddItem(service.AddFavoriteItemInput{
		UserID:    c.GetUint("userID"),
		FolderID:  req.FolderID,
		ProductID: req.ProductID,
	}); err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "add favorite item success", nil)
}

// @Summary      取消收藏
// @Description  从收藏夹中移除商品
// @Tags         收藏
// @Accept       json
// @Produce      json
// @Param        body  body  dto.FavoriteItemDeleteReq  true  "收藏夹ID和商品ID"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/favorite/items [delete]
func (h *FavoriteHandler) DeleteItem(c *gin.Context) {
	var req dto.FavoriteItemDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", err)
		return
	}
	if err := h.svc.DeleteItem(service.DeleteFavoriteItemInput{
		UserID:    c.GetUint("userID"),
		FolderID:  req.FolderID,
		ProductID: req.ProductID,
	}); err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "delete favorite item success", nil)
}

// @Summary      收藏夹商品列表
// @Tags         收藏
// @Accept       json
// @Produce      json
// @Param        body  body  dto.FavoriteItemListReq  true  "收藏夹ID和分页"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/favorite/items [get]
func (h *FavoriteHandler) ListItems(c *gin.Context) {
	// GET 接口使用查询参数（浏览器无法为 GET 携带 JSON body）
	folderID, _ := strconv.ParseUint(c.Query("folder_id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	result, err := h.svc.ListItems(service.ListFavoriteItemsInput{
		UserID:   c.GetUint("userID"),
		FolderID: uint(folderID),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "list favorite items success", result)
}

// @Summary      检查是否已收藏
// @Tags         收藏
// @Accept       json
// @Produce      json
// @Param        body  body  dto.FavoriteCheckReq  true  "商品ID"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/favorite/check [get]
func (h *FavoriteHandler) Check(c *gin.Context) {
	productID, _ := strconv.ParseUint(c.Query("product_id"), 10, 64)
	if productID == 0 {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", nil)
		return
	}
	favorited, err := h.svc.IsProductFavorited(c.GetUint("userID"), uint(productID))
	if err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "check favorite success", gin.H{
		"favorited": favorited,
	})
}
