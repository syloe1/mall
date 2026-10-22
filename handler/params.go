package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"mall/pkg/core"
)

// 分页统一校验：page ≥ 1，1 ≤ page_size ≤ 100。
// 审计问题：page=0/-1、page_size=0/9999 都被接受并静默返回第一页。
func ParsePagination(c *gin.Context) (page int, pageSize int, err error) {
	page, pageSize = 1, 20

	if raw := c.Query("page"); raw != "" {
		value, parseErr := strconv.Atoi(raw)
		if parseErr != nil || value < 1 {
			return 0, 0, core.NewBizError(http.StatusBadRequest, "page 必须是 ≥1 的整数")
		}
		page = value
	}
	if raw := c.Query("page_size"); raw != "" {
		value, parseErr := strconv.Atoi(raw)
		if parseErr != nil || value < 1 || value > 100 {
			return 0, 0, core.NewBizError(http.StatusBadRequest, "page_size 必须是 1-100 的整数")
		}
		pageSize = value
	}
	return page, pageSize, nil
}

// ParseOptionalUintQuery 解析可选的 uint query 参数，非法时报错（而不是静默当成 0）。
func ParseOptionalUintQuery(c *gin.Context, name string) (uint, error) {
	raw := c.Query(name)
	if raw == "" || raw == "0" {
		return 0, nil
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, core.NewBizError(http.StatusBadRequest, name+" 必须是非负整数")
	}
	return uint(value), nil
}
