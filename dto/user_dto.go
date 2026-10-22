package dto

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// 刷新 Token
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type UpdateUserRequest struct {
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=72"`
}

type SetPayPasswordRequest struct {
	PayPassword string `json:"pay_password" binding:"required,min=6,max=72"`
}

// 用户余额
type UserAmountResponse struct {
	Amount string `json:"amount"`
}

// 当前登录用户资料（不含密码）
type UserProfileResponse struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Role     string `json:"role"`
}

// 登录成功返回的 Token
type AuthTokensResponse struct {
	/*
		access_token：接口鉴权（前端只放内存）
		refresh_token：仅兼容 curl/Swagger 客户端；浏览器走 HttpOnly Cookie，不再下发
		role：角色（user/admin）
	*/
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Role         string `json:"role"`
}
