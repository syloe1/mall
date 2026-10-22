package middleware

import (
	"errors"
	"mall/pkg/core"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserId               uint   `json:"user_id"`    // 用户ID
	Username             string `json:"username"`   // 用户名
	Role                 string `json:"role"`       // 角色（普通用户/admin）
	TokenType            string `json:"token_type"` // token类型（access/refresh）
	jwt.RegisteredClaims        // JWT 标准字段（过期时间、签发时间等）
}

func JWTAuth(secret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			core.Fail(c, http.StatusUnauthorized, core.MsgUnauthorized, errors.New("未登录"))
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			// 没有 Bearer 前缀，直接使用原始值作为 token
			tokenString = strings.TrimSpace(tokenString)
			if tokenString == "" {
				core.Fail(c, http.StatusUnauthorized, core.MsgUnauthorized, errors.New("未登录"))
				c.Abort()
				return
			}
		}
		claims := &Claims{}
		//解析并校验 Token
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return secret, nil
		})
		//Token 无效 / 过期 → 拦截
		if err != nil || !token.Valid {
			core.Fail(c, http.StatusUnauthorized, core.MsgUnauthorized, err)
			c.Abort()
			return
		}
		//必须是 access token（防止用刷新 token 访问接口）
		if claims.TokenType != "access" {
			core.Fail(c, http.StatusUnauthorized, core.MsgUnauthorized, errors.New("refresh token 不能访问该接口"))
			c.Abort()
			return
		}
		//把用户信息放进 Gin Context
		c.Set("userID", claims.UserId)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}
