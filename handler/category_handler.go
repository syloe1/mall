package handler

import (
	"github.com/gin-gonic/gin"

	"mall/dto"
	"mall/pkg/core"
	"mall/service"
)

type CategoryHandler struct {
	svc *service.CategoryService
}

func NewCategoryHandler(svc *service.CategoryService) *CategoryHandler {
	return &CategoryHandler{svc: svc}
}

// @Summary      商品分类列表
// @Description  返回商品分类字典（id/name），前端不再硬编码
// @Tags         商品
// @Produce      json
// @Success      200  {object}  core.Response
// @Router       /categories [get]
func (h *CategoryHandler) List(c *gin.Context) {
	categories, err := h.svc.List()
	if err != nil {
		core.FailByError(c, err)
		return
	}
	items := make([]dto.CategoryResponse, 0, len(categories))
	for _, category := range categories {
		items = append(items, dto.CategoryResponse{ID: category.ID, Name: category.Name})
	}
	core.SuccessWithMessage(c, "获取分类列表成功", items)
}
