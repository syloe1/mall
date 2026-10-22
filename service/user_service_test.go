package service

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	mt "github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"mall/mock"
	"mall/model"
	"mall/pkg/aes"
	"mall/pkg/core"
)

const testPassword = "Passw0rd123"

func initTestSvc() (*UserService, *mock.MockUserDB, *mock.MockJwtCfg, *mock.MockRedisClient) {
	userDB := new(mock.MockUserDB)
	jwtCfg := new(mock.MockJwtCfg)
	rdb := new(mock.MockRedisClient)
	svc := NewUserService(userDB, jwtCfg, []byte("test-jwt-secret-12345678"), rdb)
	return svc, userDB, jwtCfg, rdb
}

func assertTokenType(t *testing.T, svc *UserService, tokenString string, want string) {
	t.Helper()
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return svc.getJWTKey(), nil
	})
	assert.NoError(t, err)
	assert.True(t, token.Valid)
	assert.Equal(t, want, claims.TokenType)
	assert.NotEmpty(t, claims.ID, "refresh token 必须带 jti，供登出吊销使用")
}

func revokedKeyMatcher() interface{} {
	return mt.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, "auth:revoked:jti:")
	})
}

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{name: "too_short", password: "Ab1", wantErr: true},
		{name: "no_digit", password: "abcdefghij", wantErr: true},
		{name: "no_letter", password: "1234567890", wantErr: true},
		{name: "common_weak", password: "password1", wantErr: true},
		{name: "demo_password_ok", password: "Demo@123456"},
		{name: "admin_password_ok", password: "Admin@123456"},
		{name: "long_random_ok", password: "mall-Go-2026-secure"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePassword(tc.password)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUserServiceRegister(t *testing.T) {
	t.Run("weak_password_rejected", func(t *testing.T) {
		svc, userDB, _, _ := initTestSvc()
		assert.Error(t, svc.Register("alice", "123"))
		assert.Error(t, svc.Register("alice", "12345678"))
		userDB.AssertNotCalled(t, "Create", mt.Anything)
	})

	t.Run("success_sets_defaults", func(t *testing.T) {
		svc, userDB, _, _ := initTestSvc()
		userDB.On("GetByUsername", "alice").Return(nil, gorm.ErrRecordNotFound).Once()
		userDB.On("Create", mt.MatchedBy(func(u *model.User) bool {
			if u.Username != "alice" || u.Nickname != "user_alice" || u.Avatar != model.DefaultAvator ||
				u.Role != model.RoleUser || u.Amount == "" {
				return false
			}
			if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(testPassword)) != nil {
				return false
			}
			costValue, err := bcrypt.Cost([]byte(u.Password))
			return err == nil && costValue == cost
		})).Return(nil).Once()
		assert.NoError(t, svc.Register("alice", testPassword))
		userDB.AssertExpectations(t)
	})

	t.Run("user_exists", func(t *testing.T) {
		svc, userDB, _, _ := initTestSvc()
		userDB.On("GetByUsername", "alice").Return(&model.User{}, nil).Once()
		assert.ErrorIs(t, svc.Register("alice", testPassword), ErrUserExists)
		userDB.AssertExpectations(t)
	})

	t.Run("query_or_create_failed", func(t *testing.T) {
		svc, userDB, _, _ := initTestSvc()
		dbErr := errors.New("db down")
		userDB.On("GetByUsername", "bob").Return(nil, dbErr).Once()
		assert.ErrorIs(t, svc.Register("bob", testPassword), dbErr)

		svc, userDB, _, _ = initTestSvc()
		createErr := errors.New("create failed")
		userDB.On("GetByUsername", "bob").Return(nil, gorm.ErrRecordNotFound).Once()
		userDB.On("Create", mt.AnythingOfType("*model.User")).Return(createErr).Once()
		assert.ErrorIs(t, svc.Register("bob", testPassword), createErr)
		userDB.AssertExpectations(t)
	})
}

func TestUserServiceLoginAndRefreshToken(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
	user := &model.User{Model: gorm.Model{ID: 1}, Username: "login_user", Password: string(hash), Role: model.RoleUser}

	t.Run("login_success", func(t *testing.T) {
		svc, userDB, _, rdb := initTestSvc()
		userDB.On("GetByUsername", user.Username).Return(user, nil).Once()
		rdb.On("Set", mt.Anything, svc.refreshTokenKey(user.ID), mt.Anything, refreshTokenTTL).
			Return(redis.NewStatusResult("OK", nil)).Once()
		access, refresh, role, err := svc.Login(user.Username, testPassword)
		assert.NoError(t, err)
		assert.Equal(t, model.RoleUser, role)
		assertTokenType(t, svc, access, tokenTypeAccess)
		assertTokenType(t, svc, refresh, tokenTypeRefresh)
		userDB.AssertExpectations(t)
		rdb.AssertExpectations(t)
	})

	t.Run("login_does_not_enumerate_users", func(t *testing.T) {
		// 用户不存在 / 密码错误必须返回完全一致的 code + msg，避免用户名枚举
		svc, userDB, _, _ := initTestSvc()
		userDB.On("GetByUsername", "missing").Return(nil, gorm.ErrRecordNotFound).Once()
		_, _, _, missingErr := svc.Login("missing", testPassword)

		svc2, userDB2, _, _ := initTestSvc()
		userDB2.On("GetByUsername", user.Username).Return(user, nil).Once()
		_, _, _, wrongErr := svc2.Login(user.Username, "WrongPassw0rd")

		var biz1, biz2 *core.BizError
		assert.ErrorAs(t, missingErr, &biz1)
		assert.ErrorAs(t, wrongErr, &biz2)
		assert.Equal(t, http.StatusUnauthorized, biz1.Code)
		assert.Equal(t, biz1.Code, biz2.Code)
		assert.Equal(t, biz1.Msg, biz2.Msg)
	})

	t.Run("login_failures", func(t *testing.T) {
		svc, userDB, _, _ := initTestSvc()
		userDB.On("GetByUsername", "missing").Return(nil, gorm.ErrRecordNotFound).Once()
		_, _, _, err := svc.Login("missing", testPassword)
		assert.Error(t, err)

		svc, userDB, _, _ = initTestSvc()
		userDB.On("GetByUsername", user.Username).Return(user, nil).Once()
		_, _, _, err = svc.Login(user.Username, "wrong")
		assert.Error(t, err)

		svc, userDB, _, rdb := initTestSvc()
		userDB.On("GetByUsername", user.Username).Return(user, nil).Once()
		rdb.On("Set", mt.Anything, svc.refreshTokenKey(user.ID), mt.Anything, refreshTokenTTL).
			Return(redis.NewStatusResult("", errors.New("redis down"))).Once()
		_, _, _, err = svc.Login(user.Username, testPassword)
		assert.Error(t, err)
	})

	t.Run("refresh_success", func(t *testing.T) {
		svc, userDB, _, rdb := initTestSvc()
		oldRefresh, _ := svc.signToken(user, tokenTypeRefresh, refreshTokenTTL)
		key := svc.refreshTokenKey(user.ID)
		rdb.On("Get", mt.Anything, revokedKeyMatcher()).Return(redis.NewStringResult("", redis.Nil)).Once()
		rdb.On("Get", mt.Anything, key).Return(redis.NewStringResult(oldRefresh, nil)).Once()
		userDB.On("FindById", user.ID).Return(user, nil).Once()
		rdb.On("Set", mt.Anything, key, mt.Anything, refreshTokenTTL).Return(redis.NewStatusResult("OK", nil)).Once()
		access, refresh, role, err := svc.RefreshToken(oldRefresh)
		assert.NoError(t, err)
		assert.Equal(t, model.RoleUser, role)
		assert.NotEmpty(t, refresh)
		assertTokenType(t, svc, access, tokenTypeAccess)
		assertTokenType(t, svc, refresh, tokenTypeRefresh)
	})

	t.Run("refresh_rejects_revoked_jti", func(t *testing.T) {
		svc, _, _, rdb := initTestSvc()
		oldRefresh, _ := svc.signToken(user, tokenTypeRefresh, refreshTokenTTL)
		rdb.On("Get", mt.Anything, revokedKeyMatcher()).Return(redis.NewStringResult("1", nil)).Once()
		_, _, _, err := svc.RefreshToken(oldRefresh)
		var biz *core.BizError
		assert.ErrorAs(t, err, &biz)
		assert.Equal(t, http.StatusUnauthorized, biz.Code)
	})

	t.Run("refresh_failures", func(t *testing.T) {
		svc, _, _, _ := initTestSvc()
		_, _, _, err := svc.RefreshToken(" ")
		assert.Error(t, err)

		access, _ := svc.signToken(user, tokenTypeAccess, accessTokenTTL)
		_, _, _, err = svc.RefreshToken(access)
		assert.Error(t, err)

		oldRefresh, _ := svc.signToken(user, tokenTypeRefresh, refreshTokenTTL)
		svc, _, _, rdb := initTestSvc()
		rdb.On("Get", mt.Anything, revokedKeyMatcher()).Return(redis.NewStringResult("", redis.Nil)).Once()
		rdb.On("Get", mt.Anything, svc.refreshTokenKey(user.ID)).Return(redis.NewStringResult("", redis.Nil)).Once()
		_, _, _, err = svc.RefreshToken(oldRefresh)
		assert.Error(t, err)
	})
}

func TestUserServiceLogout(t *testing.T) {
	user := &model.User{Model: gorm.Model{ID: 9}, Username: "logout_user", Role: model.RoleUser}
	svc, _, _, rdb := initTestSvc()
	refresh, _ := svc.signToken(user, tokenTypeRefresh, refreshTokenTTL)

	rdb.On("Set", mt.Anything, svc.refreshTokenKey(user.ID), "", mt.Anything).
		Return(redis.NewStatusResult("OK", nil)).Once()
	rdb.On("Set", mt.Anything, revokedKeyMatcher(), "1", mt.Anything).
		Return(redis.NewStatusResult("OK", nil)).Once()

	assert.NoError(t, svc.Logout(refresh))
	rdb.AssertExpectations(t)

	// 幂等：空 token 不报错
	assert.NoError(t, svc.Logout(""))
}

func TestUserServiceProfilePasswordAndAmount(t *testing.T) {
	t.Run("update_user", func(t *testing.T) {
		svc, userDB, _, _ := initTestSvc()
		u := &model.User{Model: gorm.Model{ID: 2}, Nickname: "old"}
		userDB.On("FindById", uint(2)).Return(u, nil).Once()
		userDB.On("Update", mt.MatchedBy(func(user *model.User) bool { return user.Nickname == "new" })).Return(nil).Once()
		assert.NoError(t, svc.UpdateUser(2, "new", ""))

		assert.Error(t, svc.UpdateUser(2, "", ""))
	})

	t.Run("set_pay_password", func(t *testing.T) {
		svc, userDB, _, _ := initTestSvc()
		u := &model.User{Model: gorm.Model{ID: 3}}
		userDB.On("FindById", uint(3)).Return(u, nil).Once()
		userDB.On("Update", mt.MatchedBy(func(user *model.User) bool {
			return user.PayPassword != "123456" &&
				bcrypt.CompareHashAndPassword([]byte(user.PayPassword), []byte("123456")) == nil
		})).Return(nil).Once()
		assert.NoError(t, svc.SetPayPassword(3, "123456"))
		assert.Error(t, svc.SetPayPassword(3, "123"))
	})

	t.Run("change_password", func(t *testing.T) {
		svc, userDB, _, rdb := initTestSvc()
		hash, _ := bcrypt.GenerateFromPassword([]byte("OldPassw0rd"), cost)
		u := &model.User{Model: gorm.Model{ID: 4}, Password: string(hash)}
		userDB.On("FindById", uint(4)).Return(u, nil).Once()
		userDB.On("Update", mt.AnythingOfType("*model.User")).Return(nil).Once()
		rdb.On("Set", mt.Anything, svc.refreshTokenKey(4), "", mt.Anything).
			Return(redis.NewStatusResult("OK", nil)).Once()
		assert.NoError(t, svc.ChangePassword(4, "OldPassw0rd", "NewPassw0rd"))

		// 新密码太弱 → 拒绝
		svc, userDB, _, _ = initTestSvc()
		userDB.On("FindById", uint(4)).Return(u, nil).Once()
		assert.Error(t, svc.ChangePassword(4, "OldPassw0rd", "123456"))

		svc, userDB, _, _ = initTestSvc()
		userDB.On("FindById", uint(4)).Return(u, nil).Once()
		assert.Error(t, svc.ChangePassword(4, "wrong", "NewPassw0rd"))
	})

	t.Run("get_amount", func(t *testing.T) {
		svc, userDB, _, _ := initTestSvc()
		amount, _ := aes.Encrypt("10000.00")
		userDB.On("FindById", uint(5)).Return(&model.User{Model: gorm.Model{ID: 5}, Amount: amount}, nil).Once()
		got, err := svc.GetAmount(5)
		assert.NoError(t, err)
		assert.Equal(t, "10000.00", got)

		svc, userDB, _, _ = initTestSvc()
		userDB.On("FindById", uint(5)).Return(&model.User{Model: gorm.Model{ID: 5}, Amount: "bad"}, nil).Once()
		_, err = svc.GetAmount(5)
		assert.Error(t, err)
	})
}
