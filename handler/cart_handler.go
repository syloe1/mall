package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"mall/dto"
	"mall/pkg/core"
	"mall/service"
)

type CartHandler struct {
	svc *service.CartService
}

func NewCartHandler(svc *service.CartService) *CartHandler {
	return &CartHandler{svc: svc}
}

// @Summary      加入购物车
// @Tags         购物车
// @Accept       json
// @Produce      json
// @Param        body  body  dto.CartItemAddReq  true  "商品ID和数量"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response  "库存不足或商品已下架"
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/cart/items [post]
func (h *CartHandler) AddItem(c *gin.Context) {
	var req dto.CartItemAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", err)
		return
	}
	if err := h.svc.AddItem(service.AddCartItemInput{
		UserID:    c.GetUint("userID"),
		ProductID: req.ProductID,
		Quantity:  req.Quantity,
	}); err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "add cart item success", nil)
}

// @Summary      购物车列表
// @Tags         购物车
// @Accept       json
// @Produce      json
// @Success      200  {object}  core.Response
// @Failure      401  {object}  core.Response
// @Security     BearerAuth
// @Router       /users/cart/items [get]
func (h *CartHandler) ListItems(c *gin.Context) {
	items, err := h.svc.ListItems(c.GetUint("userID"))
	if err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "list cart items success", items)
}

// @Summary      更新购物车商品数量
// @Tags         购物车
// @Accept       json
// @Produce      json
// @Param        body  body  dto.CartItemUpdateReq  true  "商品ID和新数量"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/cart/items [put]
func (h *CartHandler) UpdateItem(c *gin.Context) {
	var req dto.CartItemUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", err)
		return
	}
	if err := h.svc.UpdateItem(service.UpdateCartItemInput{
		UserID:    c.GetUint("userID"),
		ProductID: req.ProductID,
		Quantity:  req.Quantity,
	}); err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "update cart item success", nil)
}

// @Summary      删除购物车商品
// @Tags         购物车
// @Accept       json
// @Produce      json
// @Param        body  body  dto.CartItemDeleteReq  true  "商品ID"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/cart/items [delete]
func (h *CartHandler) DeleteItem(c *gin.Context) {
	var req dto.CartItemDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", err)
		return
	}
	if err := h.svc.DeleteItem(service.DeleteCartItemInput{
		UserID:    c.GetUint("userID"),
		ProductID: req.ProductID,
	}); err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "delete cart item success", nil)
}

// @Summary      清空购物车
// @Tags         购物车
// @Accept       json
// @Produce      json
// @Success      200  {object}  core.Response
// @Failure      401  {object}  core.Response
// @Security     BearerAuth
// @Router       /users/cart [delete]
func (h *CartHandler) Clear(c *gin.Context) {
	if err := h.svc.Clear(c.GetUint("userID")); err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "clear cart success", nil)
}

// @Summary      勾选/取消勾选单个商品
// @Tags         购物车
// @Accept       json
// @Produce      json
// @Param        body  body  dto.CartItemCheckedReq  true  "商品ID和勾选状态"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/cart/items/checked [put]
func (h *CartHandler) SetItemChecked(c *gin.Context) {
	var req dto.CartItemCheckedReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", err)
		return
	}
	if err := h.svc.SetItemChecked(service.SetCartItemCheckedInput{
		UserID:    c.GetUint("userID"),
		ProductID: req.ProductID,
		Checked:   req.Checked,
	}); err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "set cart item checked success", nil)
}

// @Summary      全选/取消全选
// @Tags         购物车
// @Accept       json
// @Produce      json
// @Param        body  body  dto.CartCheckedAllReq  true  "勾选状态"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/cart/checked [put]
func (h *CartHandler) SetAllChecked(c *gin.Context) {
	var req dto.CartCheckedAllReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", err)
		return
	}
	if err := h.svc.SetAllChecked(service.SetCartAllCheckedInput{
		UserID:  c.GetUint("userID"),
		Checked: req.Checked,
	}); err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "set cart checked success", nil)
}

// @Summary      已勾选商品列表
// @Description  查询购物车中已勾选的商品（用于结算）
// @Tags         购物车
// @Accept       json
// @Produce      json
// @Success      200  {object}  core.Response
// @Failure      401  {object}  core.Response
// @Security     BearerAuth
// @Router       /users/cart/checked [get]
func (h *CartHandler) ListCheckedItems(c *gin.Context) {
	items, err := h.svc.ListCheckedItems(c.GetUint("userID"))
	if err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "list checked cart items success", items)
}
