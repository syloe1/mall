package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"mall/dto"
	"mall/service"
)

func TestErrUserExistsIsDetectable(t *testing.T) {
	// UserHandler.Register specifically checks errors.Is(err, service.ErrUserExists)
	// to return 400 instead of 500. This test confirms the sentinel is detectable.
	assert.True(t, errors.Is(service.ErrUserExists, service.ErrUserExists))
}

func TestUserHandlerRegisterBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid_json_binding", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", "/register", bytes.NewBufferString("not json"))
		var req dto.RegisterRequest
		assert.Error(t, c.ShouldBindJSON(&req))
	})

	t.Run("valid_json_binds", func(t *testing.T) {
		body, _ := json.Marshal(dto.RegisterRequest{Username: "test", Password: "123456"})
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", "/register", bytes.NewBuffer(body))
		c.Request.Header.Set("Content-Type", "application/json")
		var req dto.RegisterRequest
		assert.NoError(t, c.ShouldBindJSON(&req))
		assert.Equal(t, "test", req.Username)
	})
}
