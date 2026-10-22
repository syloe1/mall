package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"mall/pkg/core"
)

func TestAdminAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("missing_role", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		// do NOT set "role" — simulates unauthenticated request

		AdminAuth()(c)

		assert.True(t, c.IsAborted(), "request should be aborted")
		assert.Equal(t, http.StatusUnauthorized, w.Code)

		var body map[string]interface{}
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, float64(http.StatusUnauthorized), body["code"])
		assert.Equal(t, core.MsgUnauthorized, body["msg"])
	})

	t.Run("role_not_string", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("role", 123) // int, not string

		AdminAuth()(c)

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusUnauthorized, w.Code)

		var body map[string]interface{}
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, core.MsgUnauthorized, body["msg"])
	})

	t.Run("role_user_denied", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("role", "user")

		AdminAuth()(c)

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusForbidden, w.Code)

		var body map[string]interface{}
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, core.MsgForbidden, body["msg"])
	})

	t.Run("role_empty_string_denied", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("role", "")

		AdminAuth()(c)

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, core.MsgForbidden, bodyMsg(t, w.Body.Bytes()))
	})

	t.Run("role_admin_allowed", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("role", "admin")

		AdminAuth()(c)

		assert.False(t, c.IsAborted(), "admin should not be aborted")
		// no response written — gin defaults to 200 when no write occurred
	})

	t.Run("role_superadmin_allowed", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("role", "superadmin")

		AdminAuth()(c)

		assert.False(t, c.IsAborted(), "superadmin should not be aborted")
	})
}

func bodyMsg(t *testing.T, raw []byte) string {
	t.Helper()
	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(raw, &body))
	msg, _ := body["msg"].(string)
	return msg
}
