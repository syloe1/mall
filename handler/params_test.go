package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"mall/pkg/core"
)

func newParamsCtx(query string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/products"+query, nil)
	return c
}

func TestParsePagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name         string
		query        string
		wantPage     int
		wantPageSize int
		wantErr      bool
	}{
		{name: "defaults", query: "", wantPage: 1, wantPageSize: 20},
		{name: "valid", query: "?page=3&page_size=50", wantPage: 3, wantPageSize: 50},
		{name: "page_zero", query: "?page=0", wantErr: true},
		{name: "page_negative", query: "?page=-1", wantErr: true},
		{name: "page_size_zero", query: "?page_size=0", wantErr: true},
		{name: "page_size_too_large", query: "?page_size=9999", wantErr: true},
		{name: "not_a_number", query: "?page=abc", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, pageSize, err := ParsePagination(newParamsCtx(tc.query))
			if tc.wantErr {
				assert.Error(t, err)
				var biz *core.BizError
				assert.ErrorAs(t, err, &biz)
				assert.Equal(t, 400, biz.Code)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.wantPage, page)
			assert.Equal(t, tc.wantPageSize, pageSize)
		})
	}
}
