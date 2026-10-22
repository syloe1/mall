package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"mall/dto"
	"mall/pkg/core"
	"mall/service"
)

type ProductHandler struct {
	svc *service.ProductService
}

func NewProductHandler(svc *service.ProductService) *ProductHandler {
	return &ProductHandler{svc: svc}
}

// @Summary      商品列表（公开）
// @Description  分页 + 分类 + 关键词查询上架商品
// @Tags         商品
// @Accept       json
// @Produce      json
// @Param        page        query  int  false  "页码"        default(1)
// @Param        page_size   query  int  false  "每页条数"    default(20)
// @Param        category_id query  int  false  "分类ID"
// @Param        keyword     query  string  false  "搜索关键词"
// @Success      200  {object}  core.Response
// @Failure      500  {object}  core.Response
// @Router       /products [get]
func (h *ProductHandler) ListPublic(c *gin.Context) {
	page, pageSize, err := ParsePagination(c)
	if err != nil {
		core.FailByError(c, err)
		return
	}
	categoryID, err := ParseOptionalUintQuery(c, "category_id")
	if err != nil {
		core.FailByError(c, err)
		return
	}

	result, err := h.svc.ListPublic(service.ListPublicProductsInput{
		Page:       page,
		PageSize:   pageSize,
		CategoryID: categoryID,
		Keyword:    c.Query("keyword"),
	})
	if err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "获取商品列表成功", result)
}

// @Summary      商品详情（公开）
// @Description  根据商品 ID 查询上架商品详情
// @Tags         商品
// @Accept       json
// @Produce      json
// @Param        id   path  uint  true  "商品ID"
// @Success      200  {object}  core.Response
// @Failure      400  {object}  core.Response  "无效的商品ID"
// @Failure      404  {object}  core.Response  "商品不存在或已下架"
// @Router       /products/{id} [get]
func (h *ProductHandler) GetPublic(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		core.Fail(c, http.StatusBadRequest, "商品 ID 不合法", err)
		return
	}
	product, err := h.svc.GetPublic(uint(id))
	if err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "获取商品成功", product)
}

// @Summary      创建商品（管理员）
// @Description  后台创建商品
// @Tags         管理员
// @Accept       json
// @Produce      json
// @Param        body  body  dto.ProductCreateReq  true  "商品信息"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Failure      403   {object}  core.Response  "非管理员"
// @Failure      500   {object}  core.Response
// @Security     BearerAuth
// @Router       /admin/products [post]
func (h *ProductHandler) Create(c *gin.Context) {
	var req dto.ProductCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", err)
		return
	}
	input := service.CreateProductInput{
		Name:        req.Name,
		PriceCents:  req.PriceCents,
		Description: req.Description,
		Stock:       req.Stock,
		CategoryID:  req.CategoryID,
		Status:      req.Status,
		Images:      req.Images,
	}
	if err := h.svc.Create(input); err != nil {
		core.Fail(c, http.StatusInternalServerError, core.MsgInternal, err)
		return
	}
	core.SuccessWithMessage(c, "创建商品成功", nil)
}

// @Summary      商品管理列表（管理员）
// @Description  后台分页查询商品列表（含下架）
// @Tags         管理员
// @Accept       json
// @Produce      json
// @Param        page        query  int     false  "页码，从 1 开始"
// @Param        page_size   query  int     false  "每页数量"
// @Param        status      query  string  false  "状态：on_sale/off_sale/draft"
// @Param        category_id query  uint    false  "分类 ID"
// @Param        keyword     query  string  false  "商品名称关键字"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Failure      403   {object}  core.Response  "非管理员"
// @Failure      500   {object}  core.Response
// @Security     BearerAuth
// @Router       /admin/products [get]
func (h *ProductHandler) ListAdmin(c *gin.Context) {
	page, pageSize, err := ParsePagination(c)
	if err != nil {
		core.FailByError(c, err)
		return
	}
	categoryID, err := ParseOptionalUintQuery(c, "category_id")
	if err != nil {
		core.FailByError(c, err)
		return
	}
	result, err := h.svc.ListAdmin(service.ListAdminProductsInput{
		Page:       page,
		PageSize:   pageSize,
		Status:     c.Query("status"),
		CategoryID: categoryID,
		Keyword:    c.Query("keyword"),
	})
	if err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "获取商品列表成功", result)
}

// @Summary      更新商品（管理员）
// @Description  后台更新商品信息
// @Tags         管理员
// @Accept       json
// @Produce      json
// @Param        body  body  dto.ProductUpdateReq  true  "商品信息"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Failure      403   {object}  core.Response  "非管理员"
// @Failure      500   {object}  core.Response
// @Security     BearerAuth
// @Router       /admin/products/detail [put]
func (h *ProductHandler) UpdateProduct(c *gin.Context) {
	var req dto.ProductUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", err)
		return
	}
	input := service.UpdateProductInput{
		ID:          req.ID,
		Name:        req.Name,
		PriceCents:  req.PriceCents,
		Description: req.Description,
		Stock:       req.Stock,
		CategoryID:  req.CategoryID,
		Status:      req.Status,
		Images:      req.Images,
	}
	if err := h.svc.UpdateProduct(input); err != nil {
		core.Fail(c, http.StatusInternalServerError, core.MsgInternal, err)
		return
	}
	core.SuccessWithMessage(c, "更新商品成功", nil)
}

// @Summary      删除商品（管理员）
// @Description  后台删除商品及关联图片
// @Tags         管理员
// @Accept       json
// @Produce      json
// @Param        body  body  dto.ProductDeleteReq  true  "商品ID"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Failure      403   {object}  core.Response  "非管理员"
// @Failure      500   {object}  core.Response
// @Security     BearerAuth
// @Router       /admin/products/detail [delete]
func (h *ProductHandler) DeleteProduct(c *gin.Context) {
	var req dto.ProductDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Fail(c, http.StatusBadRequest, "请求参数不合法", err)
		return
	}
	if err := h.svc.DeleteProduct(req.ID); err != nil {
		core.Fail(c, http.StatusInternalServerError, core.MsgInternal, err)
		return
	}
	core.SuccessWithMessage(c, "删除商品成功", nil)
}

// @Summary      重建搜索索引（管理员）
// @Description  以 MySQL 为准，全量重建 Meilisearch 商品索引（数据补偿/对账）
// @Tags         管理员
// @Produce      json
// @Success      200  {object}  core.Response
// @Failure      401  {object}  core.Response
// @Failure      403  {object}  core.Response  "非管理员"
// @Failure      500  {object}  core.Response
// @Security     BearerAuth
// @Router       /admin/products/reindex [post]
func (h *ProductHandler) Reindex(c *gin.Context) {
	count, err := h.svc.ReindexAll()
	if err != nil {
		core.Fail(c, http.StatusInternalServerError, core.MsgInternal, err)
		return
	}
	core.SuccessWithMessage(c, "重建搜索索引成功", gin.H{"indexed": count})
}

// @Summary      搜索商品（公开）
// @Description  按关键词搜索上架商品
// @Tags         商品
// @Accept       json
// @Produce      json
// @Param        page        query  int     false  "页码"        default(1)
// @Param        page_size   query  int     false  "每页条数"    default(20)
// @Param        keyword     query  string  false  "搜索关键词"
// @Param        category_id query  int     false  "分类ID"
// @Success      200  {object}  core.Response
// @Failure      500  {object}  core.Response
// @Router       /products/search [get]
func (h *ProductHandler) SearchPublic(c *gin.Context) {
	page, pageSize, err := ParsePagination(c)
	if err != nil {
		core.FailByError(c, err)
		return
	}
	categoryID, err := ParseOptionalUintQuery(c, "category_id")
	if err != nil {
		core.FailByError(c, err)
		return
	}
	keyword := c.Query("keyword")
	if keyword == "" {
		core.BadRequest(c, "关键词不能为空", nil)
		return
	}
	result, err := h.svc.SearchPublic(service.SearchPublicProductsInput{
		Page:       page,
		PageSize:   pageSize,
		CategoryID: categoryID,
		Keyword:    keyword,
	})
	if err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "搜索成功", result)
}
