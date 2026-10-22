package middleware

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"mall/pkg/core"
)

// CORS 按环境变量白名单放行跨域请求。
//
// 规则（修复旧版“回显任意 Origin + Allow-Credentials: true”的风险）：
//   - Origin 命中白名单：回显该 Origin，加 Vary: Origin，并把预检缓存压到 10 分钟
//   - Origin 不在白名单：不返回任何 Access-Control-Allow-* 头；预检直接 403
//   - 同源请求（无 Origin）：不返回 CORS 头
//   - 默认不开启 Allow-Credentials（前端用 Authorization 头，不依赖跨域 Cookie）
func CORS(allowedOrigins []string, allowCredentials bool) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if origin != "" {
			allowed[origin] = struct{}{}
		}
	}
	const allowedMethods = "GET, POST, PUT, DELETE, OPTIONS"
	const allowedHeaders = "Content-Type, Authorization, X-Request-ID, X-Requested-With"

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			// 同源/非浏览器请求：无需 CORS
			c.Next()
			return
		}

		// 任何按 Origin 变化的响应都必须声明 Vary，避免 CDN/代理缓存串味
		c.Header("Vary", "Origin")

		if _, ok := allowed[origin]; !ok {
			// 未知来源：预检请求直接拒绝，普通请求照常执行但不放行跨域读取
			if c.Request.Method == http.MethodOptions {
				slog.Warn("cors preflight rejected",
					"request_id", GetRequestID(c),
					"origin", origin,
					"path", c.Request.URL.Path,
				)
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Next()
			return
		}

		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", allowedMethods)
		c.Header("Access-Control-Allow-Headers", allowedHeaders)
		c.Header("Access-Control-Max-Age", "600")
		if allowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// NoRoute JSON 404：未知 API 路由不再返回 text/plain 的 404 page not found。
func NoRoute() gin.HandlerFunc {
	return func(c *gin.Context) {
		core.Respond(c, http.StatusNotFound, core.MsgNotFound, gin.H{
			"path": c.Request.URL.Path,
		}, nil)
	}
}

// NoMethod JSON 405。
func NoMethod() gin.HandlerFunc {
	return func(c *gin.Context) {
		core.Respond(c, http.StatusMethodNotAllowed, "请求方法不被支持", nil, nil)
	}
}
