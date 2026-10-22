package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"mall/dao"
	"mall/model"
	"mall/pkg/aes"
	"mall/pkg/core"
	"net/http"

	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var ErrUserExists = errors.New("用户名已存在")

var (
	ErrWeakPassword    = core.NewBizError(http.StatusBadRequest, "密码至少 8 位，且必须同时包含字母和数字")
	ErrPasswordTooLong = core.NewBizError(http.StatusBadRequest, "密码长度不能超过 72 位")
	ErrInvalidUsername = core.NewBizError(http.StatusBadRequest, "用户名长度需为 3-32 位，且不能包含空格")
)

type RedisClient interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
}

const (
	// cost 提到 12：bcrypt 默认 10 在 2026 年的算力下偏弱（审计要求 cost 12 / argon2id）
	cost            = 12
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 7 * 24 * time.Hour
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"

	// weakPasswordBlacklist 常见弱口令（小写比较）
	minPasswordLength = 8
	maxPasswordLength = 72
)

// dummyPasswordHash 用于“用户不存在”时也做一次 bcrypt 比较，抹平响应时间差，
// 防止通过响应时间枚举用户名。值对应一个随机串，永远不会匹配。
var dummyPasswordHash = mustHashPassword("mall-dummy-password-for-timing-only")

func mustHashPassword(password string) []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		panic(err)
	}
	return hash
}

var weakPasswordBlacklist = map[string]struct{}{
	"12345678":   {},
	"123456789":  {},
	"1234567890": {},
	"password":   {},
	"password1":  {},
	"password123": {},
	"qwerty123":  {},
	"qwertyuiop": {},
	"11111111":   {},
	"88888888":   {},
	"abc12345":   {},
	"admin123":   {},
	"admin1234":  {},
	"root1234":   {},
	"demo1234":   {},
	"iloveyou":   {},
	"letmein1":   {},
	"welcome1":   {},
	"a123456789": {},
	"mall123456": {},
}

// ValidatePassword 服务端强制密码策略（审计要求：≥8 位 + 弱口令黑名单 + 字母数字组合）。
// 前端校验只是体验，服务端必须是唯一事实来源。
func ValidatePassword(password string) error {
	if len(password) < minPasswordLength {
		return ErrWeakPassword
	}
	if len(password) > maxPasswordLength {
		return ErrPasswordTooLong
	}
	if _, blocked := weakPasswordBlacklist[strings.ToLower(password)]; blocked {
		return ErrWeakPassword
	}
	var hasLetter, hasDigit bool
	for _, r := range password {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return ErrWeakPassword
	}
	return nil
}

// ValidateUsername 基本用户名约束，避免超长/空白用户名。
func ValidateUsername(username string) error {
	username = strings.TrimSpace(username)
	if len(username) < 3 || len(username) > 32 || strings.ContainsAny(username, " \t\r\n") {
		return ErrInvalidUsername
	}
	return nil
}

// ValidateNewPassword 修改密码时使用同一套策略，并拒绝与原密码相同。
func ValidateNewPassword(oldPassword, newPassword string) error {
	if oldPassword == newPassword {
		return core.NewBizError(http.StatusBadRequest, "新密码不能与旧密码相同")
	}
	return ValidatePassword(newPassword)
}

type UserService struct {
	db     dao.UserDB
	jwtCfg dao.JWTConfig
	jwtKey []byte
	rdb    RedisClient
}

type Claims struct {
	UserId    uint   `json:"user_id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

func NewUserService(db dao.UserDB, jwtCfg dao.JWTConfig, jwtKey []byte, rdb RedisClient) *UserService {
	return &UserService{
		db:     db,
		jwtCfg: jwtCfg,
		jwtKey: jwtKey,
		rdb:    rdb,
	}
}

func (s *UserService) getJWTKey() []byte {
	if len(s.jwtKey) > 0 {
		return s.jwtKey
	}
	if s.jwtCfg != nil {
		return s.jwtCfg.GetSecret()
	}
	return nil
}

func (s *UserService) ensureUsernameAvailable(username string) error {
	_, err := s.db.GetByUsername(username)
	if err == nil {
		return ErrUserExists
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	return err
}

func (s *UserService) Register(username, password string) error {
	username = strings.TrimSpace(username)
	if err := ValidateUsername(username); err != nil {
		return err
	}
	if err := ValidatePassword(password); err != nil {
		return err
	}
	if err := s.ensureUsernameAvailable(username); err != nil {
		return err
	}

	hashPasswd, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return err
	}

	encryptedAmount, err := aes.Encrypt("10000.00")
	if err != nil {
		return err
	}

	user := model.User{
		Username: username,
		Nickname: fmt.Sprintf("user_%s", username),
		Avatar:   model.DefaultAvator,
		Password: string(hashPasswd),
		Role:     model.RoleUser,
		Amount:   encryptedAmount,
	}

	return s.db.Create(&user)
}

// Login 登录。
// 安全要求：
//   - 用户不存在 / 密码错误统一返回 401「用户名或密码错误」，不可枚举用户名
//   - 用户不存在时也执行一次 bcrypt 比较（dummy hash），抹平时间差
func (s *UserService) Login(username, password string) (string, string, string, error) {
	username = strings.TrimSpace(username)
	user, err := s.db.GetByUsername(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password))
			return "", "", "", core.NewBizError(http.StatusUnauthorized, "用户名或密码错误")
		}
		return "", "", "", core.NewBizError(http.StatusInternalServerError, core.MsgInternal)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", "", "", core.NewBizError(http.StatusUnauthorized, "用户名或密码错误")
	}

	accessToken, err := s.signToken(user, tokenTypeAccess, accessTokenTTL)
	if err != nil {
		return "", "", "", core.NewBizError(http.StatusInternalServerError, core.MsgInternal)
	}
	refreshToken, err := s.signToken(user, tokenTypeRefresh, refreshTokenTTL)
	if err != nil {
		return "", "", "", core.NewBizError(http.StatusInternalServerError, core.MsgInternal)
	}
	if err := s.saveRefreshToken(user.ID, refreshToken); err != nil {
		return "", "", "", core.NewBizError(http.StatusInternalServerError, core.MsgInternal)
	}
	return accessToken, refreshToken, user.Role, nil
}

func (s *UserService) RefreshToken(refreshToken string) (string, string, string, error) {
	tokenString := strings.TrimSpace(refreshToken)
	if tokenString == "" {
		return "", "", "", core.NewBizError(http.StatusUnauthorized, "登录状态已失效，请重新登录")
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return s.getJWTKey(), nil
	})
	if err != nil || !token.Valid {
		return "", "", "", core.NewBizError(http.StatusUnauthorized, "登录状态已失效，请重新登录")
	}
	if claims.TokenType != tokenTypeRefresh {
		return "", "", "", core.NewBizError(http.StatusUnauthorized, "登录状态已失效，请重新登录")
	}
	if s.rdb == nil {
		return "", "", "", core.NewBizError(http.StatusInternalServerError, core.MsgInternal)
	}
	// 黑名单校验（登出吊销）
	if s.isTokenRevoked(claims.ID) {
		return "", "", "", core.NewBizError(http.StatusUnauthorized, "登录状态已失效，请重新登录")
	}

	stored, err := s.rdb.Get(context.Background(), s.refreshTokenKey(claims.UserId)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", "", "", core.NewBizError(http.StatusUnauthorized, "登录状态已失效，请重新登录")
		}
		return "", "", "", core.NewBizError(http.StatusInternalServerError, core.MsgInternal)
	}
	if stored != tokenString {
		return "", "", "", core.NewBizError(http.StatusUnauthorized, "登录状态已失效，请重新登录")
	}
	user, err := s.loadUserByID(claims.UserId)
	if err != nil {
		return "", "", "", err
	}

	newAccess, err := s.signToken(user, tokenTypeAccess, accessTokenTTL)
	if err != nil {
		return "", "", "", err
	}
	newRefresh, err := s.signToken(user, tokenTypeRefresh, refreshTokenTTL)
	if err != nil {
		return "", "", "", err
	}
	if err := s.saveRefreshToken(user.ID, newRefresh); err != nil {
		return "", "", "", err
	}

	return newAccess, newRefresh, user.Role, nil
}

// Logout 登出：吊销 refresh token（删除 Redis 存储 + jti 黑名单），幂等。
func (s *UserService) Logout(refreshToken string) error {
	tokenString := strings.TrimSpace(refreshToken)
	if tokenString == "" || s.rdb == nil {
		return nil
	}
	claims := &Claims{}
	// 即使 token 已过期也要能解析出 user_id / jti 做吊销
	if _, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return s.getJWTKey(), nil
	}, jwt.WithoutClaimsValidation()); err != nil {
		return nil
	}
	if claims.TokenType != tokenTypeRefresh || claims.UserId == 0 {
		return nil
	}
	// 删除 refresh token 存储（置空并快速过期，兼容旧的 RedisClient 接口）
	if err := s.rdb.Set(context.Background(), s.refreshTokenKey(claims.UserId), "", time.Second).Err(); err != nil {
		return err
	}
	// jti 黑名单：TTL 取令牌剩余有效期，最多 7 天
	ttl := refreshTokenTTL
	if claims.ExpiresAt != nil {
		if remaining := time.Until(claims.ExpiresAt.Time); remaining > 0 && remaining < ttl {
			ttl = remaining
		}
	}
	_ = s.rdb.Set(context.Background(), s.revokedTokenKey(claims.ID), "1", ttl).Err()
	return nil
}

func (s *UserService) signToken(user *model.User, tokenType string, ttl time.Duration) (string, error) {
	claims := &Claims{
		UserId:    user.ID,
		Username:  user.Username,
		Role:      user.Role,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        newTokenID(), // jti：用于 refresh token 吊销黑名单
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.getJWTKey())
}

func newTokenID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// 保存 RefreshToken 到 Redis（登录、刷新Token时都会调用）
func (s *UserService) saveRefreshToken(userID uint, refreshToken string) error {
	if s.rdb == nil {
		return core.NewBizError(http.StatusInternalServerError, "Redis 未初始化")
	}
	return s.rdb.Set(
		context.Background(),
		s.refreshTokenKey(userID),
		refreshToken,
		refreshTokenTTL,
	).Err()
}

func (s *UserService) isTokenRevoked(jti string) bool {
	if jti == "" || s.rdb == nil {
		return false
	}
	_, err := s.rdb.Get(context.Background(), s.revokedTokenKey(jti)).Result()
	return err == nil
}

// 生成Redis Key用来存储用户的刷新令牌
func (s *UserService) refreshTokenKey(userID uint) string {
	return fmt.Sprintf("auth:refresh:user:%d", userID)
}

// 吊销令牌黑名单 key
func (s *UserService) revokedTokenKey(jti string) string {
	return "auth:revoked:jti:" + jti
}

func (s *UserService) UpdateUser(currentUserID uint, nickname string, avatar string) error {
	if nickname == "" && avatar == "" {
		return core.NewBizError(http.StatusBadRequest, "昵称或头像至少填一项")
	}

	user, err := s.loadUserByID(currentUserID)
	if err != nil {
		return err
	}

	if nickname != "" {
		user.Nickname = nickname
	}
	if avatar != "" {
		user.Avatar = avatar
	}

	if err := s.db.Update(user); err != nil {
		return core.NewBizError(http.StatusInternalServerError, "更新资料失败")
	}
	return nil
}

func (s *UserService) ChangePassword(currentUserID uint, oldPassword string, newPassword string) error {
	user, err := s.loadUserByID(currentUserID)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword)); err != nil {
		return core.NewBizError(http.StatusBadRequest, "旧密码不正确")
	}
	if err := ValidateNewPassword(oldPassword, newPassword); err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), cost)
	if err != nil {
		return core.NewBizError(http.StatusInternalServerError, core.MsgInternal)
	}

	user.Password = string(hash)
	if err := s.db.Update(user); err != nil {
		return core.NewBizError(http.StatusInternalServerError, "修改密码失败")
	}
	// 修改密码后吊销已有 refresh token（强制重新登录）
	if s.rdb != nil {
		_ = s.rdb.Set(context.Background(), s.refreshTokenKey(currentUserID), "", time.Second).Err()
	}
	return nil
}

// 设置/修改支付密码（支付、提现、交易时使用）
func (s *UserService) SetPayPassword(currentUserID uint, payPassword string) error {
	if len(payPassword) < 6 {
		return core.NewBizError(http.StatusBadRequest, "支付密码至少 6 位")
	}

	user, err := s.loadUserByID(currentUserID)
	if err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(payPassword), cost)
	if err != nil {
		return core.NewBizError(http.StatusInternalServerError, core.MsgInternal)
	}

	user.PayPassword = string(hash)

	if err := s.db.Update(user); err != nil {
		return core.NewBizError(http.StatusInternalServerError, "设置支付密码失败")
	}

	return nil
}

func (s *UserService) loadUserByID(userID uint) (*model.User, error) {
	user, err := s.db.FindById(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.NewBizError(http.StatusNotFound, "用户不存在")
		}
		return nil, core.NewBizError(http.StatusInternalServerError, core.MsgInternal)
	}
	return user, nil
}

// GetProfile 返回当前登录用户的基础资料（不含密码字段）。
func (s *UserService) GetProfile(userID uint) (*model.User, error) {
	return s.loadUserByID(userID)
}

func (s *UserService) GetAmount(userID uint) (string, error) {
	user, err := s.loadUserByID(userID)
	if err != nil {
		return "", err
	}
	amount, err := aes.Decrypt(user.Amount)
	if err != nil {
		return "", core.NewBizError(http.StatusInternalServerError, "余额解密失败")
	}
	return amount, nil
}
