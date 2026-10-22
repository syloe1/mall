package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"mall/pkg/core"
	"mall/service"
)

type DeadLetterHandler struct {
	svc *service.DeadLetterService
}

func NewDeadLetterHandler(svc *service.DeadLetterService) *DeadLetterHandler {
	return &DeadLetterHandler{svc: svc}
}

// @Summary      商品死信列表（管理员）
// @Description  查询商品事件处理失败的死信记录
// @Tags         管理员
// @Accept       json
// @Produce      json
// @Param        page        query  int     false  "页码"        default(1)
// @Param        page_size   query  int     false  "每页条数"    default(20)
// @Param        source      query  string  false  "来源(publisher/consumer)"
// @Param        event_type  query  string  false  "事件类型"
// @Param        product_id  query  int     false  "商品ID"
// @Success      200  {object}  core.Response
// @Failure      401  {object}  core.Response
// @Failure      403  {object}  core.Response  "非管理员"
// @Failure      500  {object}  core.Response
// @Security     BearerAuth
// @Router       /admin/dead-letters/products [get]
func (h *DeadLetterHandler) ListProductDeadLetters(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	productID64, _ := strconv.ParseUint(c.DefaultQuery("product_id", "0"), 10, 64)

	result, err := h.svc.ListProductDeadLetters(service.ListProductDeadLettersInput{
		Page:      page,
		PageSize:  pageSize,
		Source:    c.Query("source"),
		EventType: c.Query("event_type"),
		ProductID: uint(productID64),
	})
	if err != nil {
		core.Fail(c, http.StatusInternalServerError, core.MsgInternal, err)
		return
	}
	core.SuccessWithMessage(c, "获取商品死信列表成功", result)
}
