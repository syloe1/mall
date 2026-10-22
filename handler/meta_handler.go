package handler

import (
	"github.com/gin-gonic/gin"

	"mall/pkg/core"
)

// MetaHandler 暴露前端需要的运行时元信息（演示模式、版本等）。
type MetaHandler struct {
	demoMode bool
	version  string
}

func NewMetaHandler(demoMode bool, version string) *MetaHandler {
	return &MetaHandler{demoMode: demoMode, version: version}
}

// @Summary      运行时元信息
// @Description  前端据此决定是否展示演示账号提示 / 模拟支付入口
// @Tags         系统
// @Produce      json
// @Success      200  {object}  core.Response
// @Router       /meta [get]
func (h *MetaHandler) Get(c *gin.Context) {
	core.Success(c, gin.H{
		"demo_mode": h.demoMode,
		"version":   h.version,
	})
}
