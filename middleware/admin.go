package middleware

import (
	"errors"
	"mall/model"
	"mall/pkg/core"
	"net/http"

	"github.com/gin-gonic/gin"
)

/*
只有管理员 / 超级管理员 才能访问接口
普通用户访问 → 直接拦截，返回 401 / 403 错误
*/
//type HandlerFunc func(*Context)
//
func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleValue, ok := c.Get("role")
		if !ok {
			// 没有角色 → 未登录
			core.Fail(c, http.StatusUnauthorized, core.MsgUnauthorized, errors.New("missing role"))
			c.Abort()
			return
		}

		// 2. 把角色转成字符串
		role, ok := roleValue.(string)
		if !ok {
			core.Fail(c, http.StatusUnauthorized, core.MsgUnauthorized, errors.New("invalid role"))
			c.Abort()
			return
		}
		if role != model.RoleAdmin && role != model.RoleSuperAdmin {
			core.Fail(c, http.StatusForbidden, core.MsgForbidden, errors.New("admin permission required"))
			c.Abort()
			return
		}

		c.Next()
	}
}
