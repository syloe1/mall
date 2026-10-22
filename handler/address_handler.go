package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"mall/dto"
	"mall/pkg/core"
	"mall/service"
)

type AddressHandler struct {
	svc *service.AddressService
}

func NewAddressHandler(svc *service.AddressService) *AddressHandler {
	return &AddressHandler{svc: svc}
}

// @Summary      新增地址
// @Tags         地址
// @Accept       json
// @Produce      json
// @Param        body  body  dto.AddressCreateReq  true  "地址信息"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/addresses [post]
func (h *AddressHandler) Create(c *gin.Context) {
	var req dto.AddressCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", err)
		return
	}
	if err := h.svc.Create(service.CreateAddressInput{
		UserID:       c.GetUint("userID"),
		ReceiverName: req.ReceiverName,
		Phone:        req.Phone,
		Province:     req.Province,
		City:         req.City,
		District:     req.District,
		Detail:       req.Detail,
		PostalCode:   req.PostalCode,
		IsDefault:    req.IsDefault,
	}); err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "成功创建地址", nil)
}

// @Summary      地址列表
// @Tags         地址
// @Accept       json
// @Produce      json
// @Success      200  {object}  core.Response
// @Failure      401  {object}  core.Response
// @Security     BearerAuth
// @Router       /users/addresses [get]
func (h *AddressHandler) List(c *gin.Context) {
	addresses, err := h.svc.List(c.GetUint("userID"))
	if err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "地址列表", addresses)
}

// @Summary      地址详情
// @Tags         地址
// @Accept       json
// @Produce      json
// @Param        id   query  uint  true  "地址ID"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/addresses/detail [get]
func (h *AddressHandler) Get(c *gin.Context) {
	var req dto.AddressGetReq
	if err := c.ShouldBindQuery(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", err)
		return
	}
	address, err := h.svc.Get(service.GetAddressInput{
		UserID:    c.GetUint("userID"),
		AddressID: req.ID,
	})
	if err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "查看地址成功", address)
}

// @Summary      更新地址
// @Tags         地址
// @Accept       json
// @Produce      json
// @Param        body  body  dto.AddressUpdateReq  true  "地址信息"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/addresses [put]
func (h *AddressHandler) Update(c *gin.Context) {
	var req dto.AddressUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", err)
		return
	}
	if err := h.svc.Update(service.UpdateAddressInput{
		UserID:       c.GetUint("userID"),
		AddressID:    req.ID,
		ReceiverName: req.ReceiverName,
		Phone:        req.Phone,
		Province:     req.Province,
		City:         req.City,
		District:     req.District,
		Detail:       req.Detail,
		PostalCode:   req.PostalCode,
		IsDefault:    req.IsDefault,
	}); err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "update address success", nil)
}

// @Summary      删除地址
// @Tags         地址
// @Accept       json
// @Produce      json
// @Param        body  body  dto.AddressDeleteReq  true  "地址ID"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/addresses [delete]
func (h *AddressHandler) Delete(c *gin.Context) {
	var req dto.AddressDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", err)
		return
	}
	if err := h.svc.Delete(service.DeleteAddressInput{
		UserID:    c.GetUint("userID"),
		AddressID: req.ID,
	}); err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "delete address success", nil)
}

// @Summary      设为默认地址
// @Tags         地址
// @Accept       json
// @Produce      json
// @Param        body  body  dto.AddressSetDefaultReq  true  "地址ID"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/addresses/default [put]
func (h *AddressHandler) SetDefault(c *gin.Context) {
	var req dto.AddressSetDefaultReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", err)
		return
	}
	if err := h.svc.SetDefault(service.SetDefaultAddressInput{
		UserID:    c.GetUint("userID"),
		AddressID: req.ID,
	}); err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "set default address success", nil)
}
