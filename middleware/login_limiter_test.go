package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

func newLimiterCtx() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/users/login", nil)
	c.Request.RemoteAddr = "203.0.113.9:12345"
	return c, w
}

func TestLoginLimiter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	limiter := NewLoginLimiterWithConfig(rdb, 2, 60*1e9)

	t.Run("allows_within_limit", func(t *testing.T) {
		c, _ := newLimiterCtx()
		assert.True(t, limiter.Check(c, "demo"))
		assert.True(t, limiter.Check(c, "demo"))
	})

	t.Run("blocks_over_limit_with_retry_after", func(t *testing.T) {
		c, w := newLimiterCtx()
		assert.False(t, limiter.Check(c, "demo"))
		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.Equal(t, "60", w.Header().Get("Retry-After"))
		assert.Contains(t, w.Body.String(), "请求过于频繁")
	})

	t.Run("other_account_not_affected", func(t *testing.T) {
		c, _ := newLimiterCtx()
		assert.True(t, limiter.Check(c, "someone-else"))
	})

	t.Run("nil_limiter_allows", func(t *testing.T) {
		c, _ := newLimiterCtx()
		var nilLimiter *LoginLimiter
		assert.True(t, nilLimiter.Check(c, "demo"))
	})
}
