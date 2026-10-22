package middleware

import (
	"errors"
	"mall/pkg/core"
	"mall/pkg/ratelimit"
	"net/http"

	"github.com/gin-gonic/gin"
)

func TokenBucketLimit(bucket *ratelimit.TokenBucket) gin.HandlerFunc {
	return func(c *gin.Context) {
		if bucket != nil && !bucket.Allow() {
			core.Fail(c, http.StatusTooManyRequests, "请求过于频繁，请稍后再试", errors.New("rate limit exceeded"))
			c.Abort()
			return
		}
		c.Next()
	}
}
