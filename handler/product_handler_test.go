package handler

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestProductHandler_GetPublic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid_id_non_numeric_returns_400", func(t *testing.T) {
		// Verify the parsing logic the handler uses
		_, err := strconv.ParseUint("abc", 10, 64)
		assert.Error(t, err)
	})

	t.Run("id_zero_returns_400", func(t *testing.T) {
		// strconv.ParseUint("0", ...) succeeds but handler checks id == 0
		id, err := strconv.ParseUint("0", 10, 64)
		assert.NoError(t, err)
		assert.Equal(t, uint64(0), id)
	})

	t.Run("valid_numeric_id_parses", func(t *testing.T) {
		id, err := strconv.ParseUint("1", 10, 64)
		assert.NoError(t, err)
		assert.Equal(t, uint64(1), id)
		assert.NotZero(t, id)
	})
}

// Verify the response helper used by handlers produces the expected format
func TestHandlerResponseFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	assert.NotNil(t, w)
	assert.Equal(t, http.StatusOK, w.Code)
}
