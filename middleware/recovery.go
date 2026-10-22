package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"mall/pkg/core"
)

// Recovery 统一 panic 恢复：写日志（含堆栈 + request_id），对外只返回通用 500 JSON。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("panic recovered",
					"request_id", GetRequestID(c),
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
					"panic", recovered,
					"stack", string(debug.Stack()),
				)
				core.Respond(c, http.StatusInternalServerError, core.MsgInternal, nil, nil)
				c.Abort()
			}
		}()
		c.Next()
	}
}
