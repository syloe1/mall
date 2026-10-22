package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newCorsCtx(method string, origin string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, "/api/v1/products", nil)
	if origin != "" {
		c.Request.Header.Set("Origin", origin)
	}
	return c, w
}

func TestCORSWhitelist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	allowed := []string{"https://syloe1.xyz", "https://mall.syloe1.xyz"}

	t.Run("allowed_origin_echoed_with_vary", func(t *testing.T) {
		c, w := newCorsCtx("GET", "https://mall.syloe1.xyz")
		CORS(allowed, false)(c)
		assert.Equal(t, "https://mall.syloe1.xyz", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "Origin", w.Header().Get("Vary"))
		assert.Equal(t, "600", w.Header().Get("Access-Control-Max-Age"))
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Credentials"))
	})

	t.Run("unknown_origin_gets_no_cors_headers", func(t *testing.T) {
		c, w := newCorsCtx("GET", "https://evil.example.com")
		CORS(allowed, false)(c)
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "Origin", w.Header().Get("Vary"))
		assert.False(t, c.IsAborted())
	})

	t.Run("unknown_origin_preflight_forbidden", func(t *testing.T) {
		c, w := newCorsCtx(http.MethodOptions, "https://evil.example.com")
		CORS(allowed, false)(c)
		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("allowed_origin_preflight_no_content", func(t *testing.T) {
		c, w := newCorsCtx(http.MethodOptions, "https://syloe1.xyz")
		CORS(allowed, false)(c)
		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")
		assert.NotContains(t, w.Header().Get("Access-Control-Allow-Methods"), "PATCH")
	})

	t.Run("no_origin_request_untouched", func(t *testing.T) {
		c, w := newCorsCtx("GET", "")
		CORS(allowed, false)(c)
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
		assert.Empty(t, w.Header().Get("Vary"))
		assert.False(t, c.IsAborted())
	})

	t.Run("empty_whitelist_never_echoes", func(t *testing.T) {
		c, w := newCorsCtx("GET", "https://mall.syloe1.xyz")
		CORS(nil, false)(c)
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
	})
}
