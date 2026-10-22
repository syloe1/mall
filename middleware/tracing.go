package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// 把一次请求从网关 → 接口 → 调用其他服务 → DB → MQ 全程串起来
func Tracing() gin.HandlerFunc {
	tracer := otel.Tracer("mall/http")

	return func(c *gin.Context) {
		ctx := otel.GetTextMapPropagator().Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		//Span 一次调用的记录

		spanName := fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path)
		ctx, span := tracer.Start(ctx, spanName,
			oteltrace.WithSpanKind(oteltrace.SpanKindServer),
			oteltrace.WithAttributes(
				attribute.String("http.method", c.Request.Method),
				attribute.String("http.route", c.Request.URL.Path),
				attribute.String("http.user_agent", c.Request.UserAgent()),
			),
		)
		defer span.End()
		//把 Trace 信息存入上下文
		c.Request = c.Request.WithContext(ctx)
		c.Set("trace_id", span.SpanContext().TraceID().String())

		c.Next()
		//接口结束后记录状态码
		status := c.Writer.Status()
		span.SetAttributes(attribute.Int("http.status_code", status))
		//标记错误
		if status >= 500 {
			span.SetStatus(codes.Error, "http server error")
			return
		}
		if status >= 400 {
			span.SetStatus(codes.Error, "http client error")
		}
	}
}
