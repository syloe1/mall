package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// AccessLog () = 全局访问日志中间件
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		//请求开始时间
		start := time.Now()

		c.Next()
		//耗时
		latency := time.Since(start)
		status := c.Writer.Status() //状态码200/400/500
		//请求信息
		//构建日志字段

		attrs := []any{
			"request_id", GetRequestID(c), // 请求ID（追踪用）
			"method", c.Request.Method,
			"path", c.Request.URL.Path, // 请求路径 /api/user
			"status", status,
			"latency_ms", latency.Milliseconds(), // 耗时 ms
			"client_ip", c.ClientIP(), // 客户端IP
			"user_agent", c.Request.UserAgent(), // 浏览器/客户端信息
		}
		// 如果登录了，带上 user_id
		if userID, ok := c.Get("userID"); ok {
			attrs = append(attrs, "user_id", userID)
		}
		if traceID, ok := c.Get("trace_id"); ok {
			attrs = append(attrs, "trace_id", traceID)
		}

		if status >= 500 {
			slog.Error("http request", attrs...)
			return
		}
		if status >= 400 {
			slog.Warn("http request", attrs...)
			return
		}
		slog.Info("http request", attrs...)
	}
}

//INFO http request request_id=abc123 method=GET path=/api/user status=200 latency_ms=5 client_ip=127.0.0.1 user_id=1001 trace_id=xyz456

/*
记录所有接口请求，方便排查问题
统计接口耗时，做性能优化
监控接口状态码（500 告警）
全链路追踪（trace_id）
用户行为审计（userID、IP、UA）
线上出问题，靠它定位！
r := gin.Default()
r.Use(middleware.AccessLog()) // 全局注册
*/
