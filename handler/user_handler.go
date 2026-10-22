package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"mall/config"
	"mall/dto"
	"mall/middleware"
	"mall/pkg/core"
	"mall/service"
)

// refreshCookieName refresh token 的 HttpOnly Cookie 名。
// 前端 access token 只放内存，页面刷新时靠这个 Cookie 静默换新。
const refreshCookieName = "mall_refresh"

// refreshCookiePath 只对认证相关接口可见，降低被意外发送到其他接口的概率。
const refreshCookiePath = "/api/v1/users"

type UserHandler struct {
	svc     *service.UserService
	limiter *middleware.LoginLimiter
	cfg     *config.App
}

func NewUserHandler(svc *service.UserService, limiter *middleware.LoginLimiter, cfg *config.App) *UserHandler {
	return &UserHandler{svc: svc, limiter: limiter, cfg: cfg}
}

func (h *UserHandler) cookieSecure(c *gin.Context) bool {
	if h.cfg != nil && h.cfg.GetSecurityConfig().CookieSecure {
		return true
	}
	// 兼容 Cloudflare Tunnel：TLS 在边缘终止，Go 侧只能看 X-Forwarded-Proto
	return c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
}

func (h *UserHandler) setRefreshCookie(c *gin.Context, token string, maxAgeSeconds int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(refreshCookieName, token, maxAgeSeconds, refreshCookiePath, "", h.cookieSecure(c), true)
}

func (h *UserHandler) clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(refreshCookieName, "", -1, refreshCookiePath, "", h.cookieSecure(c), true)
}

func (h *UserHandler) refreshTokenFromRequest(c *gin.Context, bodyToken string) string {
	if token := strings.TrimSpace(bodyToken); token != "" {
		return token
	}
	cookie, err := c.Cookie(refreshCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie)
}

// @Summary      用户注册
// @Description  创建新用户账号（密码策略：≥8 位且同时包含字母和数字）
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        body  body  dto.RegisterRequest  true  "注册信息"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response  "参数错误 / 密码太弱 / 用户名已存在"
// @Failure      429   {object}  core.Response  "请求过于频繁"
// @Router       /users/register [post]
func (h *UserHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.BadRequest(c, "注册信息格式不正确", err)
		return
	}
	if !h.limiter.Check(c, req.Username) {
		return
	}
	if err := h.svc.Register(req.Username, req.Password); err != nil {
		if errors.Is(err, service.ErrUserExists) {
			core.Respond(c, http.StatusBadRequest, "用户名已存在", nil, nil)
			return
		}
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "注册成功", nil)
}

// @Summary      用户登录
// @Description  用户名密码登录；access token 返回给前端放内存，refresh token 写入 HttpOnly Cookie
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        body  body  dto.LoginRequest  true  "登录信息"
// @Success      200   {object}  core.Response{data=dto.AuthTokensResponse}
// @Failure      401   {object}  core.Response  "用户名或密码错误"
// @Failure      429   {object}  core.Response  "请求过于频繁"
// @Router       /users/login [post]
func (h *UserHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.BadRequest(c, "登录信息格式不正确", err)
		return
	}
	if !h.limiter.Check(c, req.Username) {
		return
	}
	accessToken, refreshToken, role, err := h.svc.Login(req.Username, req.Password)
	if err != nil {
		core.FailByError(c, err)
		return
	}
	h.setRefreshCookie(c, refreshToken, int((7 * 24 * 60 * 60)))
	// 响应体只返回 access token + 角色；refresh token 不出现在 JS 可读的位置
	core.Success(c, dto.AuthTokensResponse{AccessToken: accessToken, Role: role})
}

// @Summary      刷新 Token
// @Description  使用 HttpOnly Cookie（或请求体 refresh_token）换取新的 access token
// @Tags         用户
// @Accept       json
// @Produce      json
// @Success      200   {object}  core.Response{data=dto.AuthTokensResponse}
// @Failure      401   {object}  core.Response  "token 无效或已失效"
// @Router       /users/refresh-token [post]
func (h *UserHandler) RefreshToken(c *gin.Context) {
	var req dto.RefreshTokenRequest
	// 允许空 body（Cookie 模式），因此忽略绑定错误，只在完全拿不到 token 时报 401
	_ = c.ShouldBindJSON(&req)
	token := h.refreshTokenFromRequest(c, req.RefreshToken)
	if token == "" {
		core.Respond(c, http.StatusUnauthorized, "登录状态已失效，请重新登录", nil, nil)
		return
	}
	accessToken, newRefreshToken, role, err := h.svc.RefreshToken(token)
	if err != nil {
		core.FailByError(c, err)
		return
	}
	h.setRefreshCookie(c, newRefreshToken, int((7 * 24 * 60 * 60)))
	core.Success(c, dto.AuthTokensResponse{AccessToken: accessToken, Role: role})
}

// @Summary      登出
// @Description  吊销当前 refresh token（Redis + jti 黑名单）并清除 HttpOnly Cookie；幂等
// @Tags         用户
// @Produce      json
// @Success      200   {object}  core.Response
// @Router       /users/logout [post]
func (h *UserHandler) Logout(c *gin.Context) {
	var req dto.RefreshTokenRequest
	_ = c.ShouldBindJSON(&req)
	token := h.refreshTokenFromRequest(c, req.RefreshToken)
	if err := h.svc.Logout(token); err != nil {
		core.FailByError(c, err)
		return
	}
	h.clearRefreshCookie(c)
	core.SuccessWithMessage(c, "已退出登录", nil)
}

// @Summary      当前用户资料
// @Description  返回用户名 / 昵称 / 头像 / 角色（不含密码）
// @Tags         用户
// @Produce      json
// @Success      200   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/me [get]
func (h *UserHandler) GetProfile(c *gin.Context) {
	user, err := h.svc.GetProfile(c.GetUint("userID"))
	if err != nil {
		core.FailByError(c, err)
		return
	}
	core.Success(c, dto.UserProfileResponse{
		ID:       user.ID,
		Username: user.Username,
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
		Role:     user.Role,
	})
}

// @Summary      修改用户信息
// @Description  修改昵称和头像
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        body  body  dto.UpdateUserRequest  true  "用户信息"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/me [put]
func (h *UserHandler) Update(c *gin.Context) {
	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.BadRequest(c, "用户信息格式不正确", err)
		return
	}
	if err := h.svc.UpdateUser(c.GetUint("userID"), req.Nickname, req.Avatar); err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "资料已更新", nil)
}

// @Summary      修改登录密码
// @Description  输入旧密码和新密码（新密码 ≥8 位且含字母与数字）
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        body  body  dto.ChangePasswordRequest  true  "新旧密码"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response  "旧密码错误 / 新密码太弱"
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/me/password [put]
func (h *UserHandler) ChangePassword(c *gin.Context) {
	var req dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.BadRequest(c, "密码信息格式不正确", err)
		return
	}
	if err := h.svc.ChangePassword(c.GetUint("userID"), req.OldPassword, req.NewPassword); err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "密码已修改，请重新登录", nil)
}

// @Summary      设置支付密码
// @Description  设置/修改余额支付密码，至少6位
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        body  body  dto.SetPayPasswordRequest  true  "支付密码"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/me/pay-password [put]
func (h *UserHandler) SetPayPassword(c *gin.Context) {
	var req dto.SetPayPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.BadRequest(c, "支付密码格式不正确", err)
		return
	}
	if err := h.svc.SetPayPassword(c.GetUint("userID"), req.PayPassword); err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "支付密码已设置", nil)
}

// @Summary      查询余额
// @Description  查询当前用户的账户余额
// @Tags         用户
// @Produce      json
// @Success      200   {object}  core.Response{data=dto.UserAmountResponse}
// @Failure      401   {object}  core.Response
// @Security     BearerAuth
// @Router       /users/me/amount [get]
func (h *UserHandler) GetAmount(c *gin.Context) {
	amount, err := h.svc.GetAmount(c.GetUint("userID"))
	if err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "获取金额成功", dto.UserAmountResponse{
		Amount: amount,
	})
}
