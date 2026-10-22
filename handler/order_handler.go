package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"mall/dto"
	"mall/pkg/core"
	"mall/service"
)

type OrderHandler struct {
	svc *service.OrderService
}

func NewOrderHandler(svc *service.OrderService) *OrderHandler {
	return &OrderHandler{svc: svc}
}

// @Summary      购物车结算
// @Description  将购物车中已勾选商品生成订单
// @Tags         订单
// @Accept       json
// @Produce      json
// @Param        body  body  dto.OrderCreateFromCartReq  true  "收货地址ID"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response  "无勾选商品/库存不足"
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/orders/cart [post]
func (h *OrderHandler) CreateFromCart(c *gin.Context) {
	var req dto.OrderCreateFromCartReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", err)
		return
	}
	order, err := h.svc.CreateFromCart(service.CreateOrderFromCartInput{
		UserID:    c.GetUint("userID"),
		AddressID: req.AddressID,
	})
	if err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "create order success", order)
}

// @Summary      立即购买
// @Description  跳过购物车直接下单单个商品
// @Tags         订单
// @Accept       json
// @Produce      json
// @Param        body  body  dto.OrderBuyNowReq  true  "商品ID、数量、地址ID"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response  "库存不足"
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/orders/buy-now [post]
func (h *OrderHandler) BuyNow(c *gin.Context) {
	var req dto.OrderBuyNowReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", err)
		return
	}
	order, err := h.svc.BuyNow(service.BuyNowInput{
		UserID:    c.GetUint("userID"),
		ProductID: req.ProductID,
		Quantity:  req.Quantity,
		AddressID: req.AddressID,
	})
	if err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "create order success", order)
}

// @Summary      我的订单列表
// @Description  当前用户的订单分页列表（含商品快照），支持按状态过滤
// @Tags         订单
// @Produce      json
// @Param        page query int false "页码，默认 1"
// @Param        page_size query int false "每页数量，默认 20"
// @Param        status query string false "订单状态过滤"
// @Success      200  {object}  core.Response
// @Failure      401  {object}  core.Response
// @Security     BearerAuth
// @Router       /users/orders [get]
func (h *OrderHandler) ListOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	result, err := h.svc.ListOrders(service.ListOrdersInput{
		UserID:   c.GetUint("userID"),
		Status:   c.Query("status"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "list orders success", result)
}
