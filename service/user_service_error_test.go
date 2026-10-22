package service

import (
	"net/http"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"mall/model"
)

// ═══════════════════════════════════════════════════════════════════════════════
// ChangePassword — error paths
// ═══════════════════════════════════════════════════════════════════════════════

func TestUserService_ChangePassword_Errors(t *testing.T) {

	t.Run("user_not_found", func(t *testing.T) {
		svc, userDB, _, _ := initTestSvc()
		userDB.On("FindById", uint(1)).Return(nil, gorm.ErrRecordNotFound).Once()
		err := svc.ChangePassword(1, "old", "new123")
		mustBeBizError(t, err, http.StatusNotFound, "用户不存在")
		userDB.AssertExpectations(t)
	})

	t.Run("wrong_old_password", func(t *testing.T) {
		svc, userDB, _, _ := initTestSvc()
		hashed, _ := bcrypt.GenerateFromPassword([]byte("correct"), bcrypt.MinCost)
		userDB.On("FindById", uint(1)).Return(&model.User{Password: string(hashed)}, nil).Once()
		err := svc.ChangePassword(1, "wrong", "new123")
		mustBeBizError(t, err, http.StatusBadRequest, "旧密码不正确")
		userDB.AssertExpectations(t)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// SetPayPassword — error paths
// ═══════════════════════════════════════════════════════════════════════════════

func TestUserService_SetPayPassword_Errors(t *testing.T) {

	t.Run("user_not_found", func(t *testing.T) {
		svc, userDB, _, _ := initTestSvc()
		userDB.On("FindById", uint(1)).Return(nil, gorm.ErrRecordNotFound).Once()
		err := svc.SetPayPassword(1, "123456")
		mustBeBizError(t, err, http.StatusNotFound, "用户不存在")
		userDB.AssertExpectations(t)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// UpdateUser — error paths
// ═══════════════════════════════════════════════════════════════════════════════

func TestUserService_UpdateUser_Errors(t *testing.T) {

	t.Run("user_not_found", func(t *testing.T) {
		svc, userDB, _, _ := initTestSvc()
		userDB.On("FindById", uint(1)).Return(nil, gorm.ErrRecordNotFound).Once()
		err := svc.UpdateUser(1, "nick", "avatar.jpg")
		mustBeBizError(t, err, http.StatusNotFound, "用户不存在")
		userDB.AssertExpectations(t)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// GetAmount — error paths
// ═══════════════════════════════════════════════════════════════════════════════

func TestUserService_GetAmount_Errors(t *testing.T) {

	t.Run("user_not_found", func(t *testing.T) {
		svc, userDB, _, _ := initTestSvc()
		userDB.On("FindById", uint(1)).Return(nil, gorm.ErrRecordNotFound).Once()
		_, err := svc.GetAmount(1)
		mustBeBizError(t, err, http.StatusNotFound, "用户不存在")
		userDB.AssertExpectations(t)
	})
}
