package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"mall/dto"
	"mall/pkg/core"
	"mall/service"
)

type PaymentHandler struct {
	svc *service.PaymentService
}

func NewPaymentHandler(svc *service.PaymentService) *PaymentHandler {
	return &PaymentHandler{svc: svc}
}

// @Summary      创建支付
// @Description  对指定订单发起支付（mock/balance）
// @Tags         支付
// @Accept       json
// @Produce      json
// @Param        body  body  dto.PaymentCreateReq  true  "订单ID和支付渠道"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response  "订单已支付/状态不正确"
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/payments [post]
func (h *PaymentHandler) CreatePayment(c *gin.Context) {
	var req dto.PaymentCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", err)
		return
	}
	payment, err := h.svc.CreatePayment(service.CreatePaymentInput{
		UserID:      c.GetUint("userID"),
		OrderID:     req.OrderID,
		Channel:     req.Channel,
		PayPassword: req.PayPassword,
	})
	if err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "create payment success", payment)
}

// @Summary      查询支付结果
// @Tags         支付
// @Accept       json
// @Produce      json
// @Param        payment_no  query  string  true  "支付单号"
// @Success      200  {object}  core.Response
// @Failure      400  {object}  core.Response
// @Failure      401  {object}  core.Response
// @Security     BearerAuth
// @Router       /users/payments/detail [get]
func (h *PaymentHandler) GetPayment(c *gin.Context) {
	paymentNo := c.Query("payment_no")
	if paymentNo == "" {
		core.Fail(c, http.StatusBadRequest, "请提供支付单号", nil)
		return
	}
	payment, err := h.svc.GetPayment(service.GetPaymentInput{
		UserID:    c.GetUint("userID"),
		PaymentNo: paymentNo,
	})
	if err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "get payment success", payment)
}

// @Summary      模拟支付成功
// @Description  仅用于测试，将支付单标记为成功
// @Tags         支付
// @Accept       json
// @Produce      json
// @Param        body  body  dto.PaymentMockSuccessReq  true  "支付单号"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/payments/mock/success [post]
func (h *PaymentHandler) MockPaySuccess(c *gin.Context) {
	var req dto.PaymentMockSuccessReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", err)
		return
	}
	payment, err := h.svc.MockPaySuccess(service.MockPaySuccessInput{
		UserID:    c.GetUint("userID"),
		PaymentNo: req.PaymentNo,
	})
	if err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "pay success", payment)
}
