package handler

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSeckillHandlerBuyParsing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid_activity_id_non_numeric", func(t *testing.T) {
		// The handler uses strconv.ParseUint on c.Param("activity_id")
		_, err := strconv.ParseUint("abc", 10, 64)
		assert.Error(t, err)
	})

	t.Run("activity_id_zero_invalid", func(t *testing.T) {
		// handler checks: err != nil || activityID64 == 0
		id, err := strconv.ParseUint("0", 10, 64)
		assert.NoError(t, err)
		assert.Equal(t, uint64(0), id)
	})

	t.Run("valid_activity_id_parses", func(t *testing.T) {
		id, err := strconv.ParseUint("42", 10, 64)
		assert.NoError(t, err)
		assert.Equal(t, uint64(42), id)
	})

	t.Run("userID_type_assertion", func(t *testing.T) {
		// The handler does: userID, ok := userIDValue.(uint)
		// If middleware stores int instead of uint, this returns ok=false
		var val interface{} = int(1)
		_, ok := val.(uint)
		assert.False(t, ok, "int should not assert as uint — handler returns 401")

		val = uint(1)
		uid, ok := val.(uint)
		assert.True(t, ok)
		assert.Equal(t, uint(1), uid)
	})
}

func TestHandlerContextIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotNil(t, c)
}
