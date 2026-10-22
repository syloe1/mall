package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	mt "github.com/stretchr/testify/mock"
	"gorm.io/gorm"

	"mall/mock"
	"mall/model"
)

func initAddressTestSvc() (*AddressService, *mock.MockAddressDB) {
	db := new(mock.MockAddressDB)
	svc := NewAddressService(db)
	return svc, db
}

func input() CreateAddressInput {
	return CreateAddressInput{
		UserID:       1,
		ReceiverName: "张三",
		Phone:        "13800138000",
		Province:     "广东省",
		City:         "深圳市",
		District:     "南山区",
		Detail:       "科技园路1号",
	}
}

// ─── Create ───────────────────────────────────────────────────────────────────

func TestAddressServiceCreate(t *testing.T) {
	t.Run("success_no_existing_addresses_sets_default", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		db.On("CountByUserID", uint(1)).Return(int64(0), nil).Once()
		db.On("ClearDefault", uint(1)).Return(nil).Once()
		db.On("Create", matchedAddress(func(a *model.UserAddress) bool {
			return a.IsDefault && a.ReceiverName == "张三"
		})).Return(nil).Once()
		assert.NoError(t, svc.Create(input()))
		db.AssertExpectations(t)
	})

	t.Run("success_has_existing_not_default", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		db.On("CountByUserID", uint(1)).Return(int64(3), nil).Once()
		db.On("Create", matchedAddress(func(a *model.UserAddress) bool {
			return !a.IsDefault
		})).Return(nil).Once()
		assert.NoError(t, svc.Create(input()))
		db.AssertExpectations(t)
	})

	t.Run("success_is_default_clears_old_default", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		inp := input()
		inp.IsDefault = true
		db.On("CountByUserID", uint(1)).Return(int64(2), nil).Once()
		db.On("ClearDefault", uint(1)).Return(nil).Once()
		db.On("Create", matchedAddress(func(a *model.UserAddress) bool {
			return a.IsDefault
		})).Return(nil).Once()
		assert.NoError(t, svc.Create(inp))
		db.AssertExpectations(t)
	})

	t.Run("success_count_zero_first_is_default", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		inp := input()
		inp.IsDefault = false
		db.On("CountByUserID", uint(1)).Return(int64(0), nil).Once()
		db.On("ClearDefault", uint(1)).Return(nil).Once()
		db.On("Create", matchedAddress(func(a *model.UserAddress) bool {
			return a.IsDefault
		})).Return(nil).Once()
		assert.NoError(t, svc.Create(inp))
		db.AssertExpectations(t)
	})

	t.Run("fail_invalid_user_id", func(t *testing.T) {
		svc, _ := initAddressTestSvc()
		inp := input()
		inp.UserID = 0
		assert.Error(t, svc.Create(inp))
	})

	t.Run("fail_missing_required_field", func(t *testing.T) {
		svc, _ := initAddressTestSvc()
		for _, tc := range []struct {
			name string
			inp  CreateAddressInput
		}{
			{"missing_receiver", CreateAddressInput{UserID: 1}},
			{"missing_phone", CreateAddressInput{UserID: 1, ReceiverName: "x", Province: "x", City: "x", District: "x", Detail: "x"}},
			{"missing_province", CreateAddressInput{UserID: 1, ReceiverName: "x", Phone: "x", City: "x", District: "x", Detail: "x"}},
			{"missing_city", CreateAddressInput{UserID: 1, ReceiverName: "x", Phone: "x", Province: "x", District: "x", Detail: "x"}},
			{"missing_district", CreateAddressInput{UserID: 1, ReceiverName: "x", Phone: "x", Province: "x", City: "x", Detail: "x"}},
			{"missing_detail", CreateAddressInput{UserID: 1, ReceiverName: "x", Phone: "x", Province: "x", City: "x", District: "x"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				assert.Error(t, svc.Create(tc.inp))
			})
		}
	})

	t.Run("fail_count_exceeds_limit", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		db.On("CountByUserID", uint(1)).Return(int64(maxAddressCount), nil).Once()
		assert.Error(t, svc.Create(input()))
		db.AssertExpectations(t)
	})

	t.Run("fail_count_query_error", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		db.On("CountByUserID", uint(1)).Return(int64(0), errors.New("db err")).Once()
		assert.Error(t, svc.Create(input()))
		db.AssertExpectations(t)
	})

	t.Run("fail_clear_default_error", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		inp := input()
		inp.IsDefault = true
		db.On("CountByUserID", uint(1)).Return(int64(3), nil).Once()
		db.On("ClearDefault", uint(1)).Return(errors.New("db err")).Once()
		assert.Error(t, svc.Create(inp))
		db.AssertExpectations(t)
	})

	t.Run("fail_create_error", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		db.On("CountByUserID", uint(1)).Return(int64(0), nil).Once()
		db.On("ClearDefault", uint(1)).Return(nil).Once()
		db.On("Create", matchedAddress(nil)).Return(errors.New("db err")).Once()
		assert.Error(t, svc.Create(input()))
		db.AssertExpectations(t)
	})
}

// ─── List ─────────────────────────────────────────────────────────────────────

func TestAddressServiceList(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		want := []model.UserAddress{
			{ReceiverName: "张三"},
			{ReceiverName: "李四"},
		}
		db.On("ListByUserID", uint(1)).Return(want, nil).Once()
		got, err := svc.List(1)
		assert.NoError(t, err)
		assert.Len(t, got, 2)
		db.AssertExpectations(t)
	})

	t.Run("fail_invalid_user_id", func(t *testing.T) {
		svc, _ := initAddressTestSvc()
		_, err := svc.List(0)
		assert.Error(t, err)
	})

	t.Run("fail_db_error", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		db.On("ListByUserID", uint(1)).Return([]model.UserAddress(nil), errors.New("db err")).Once()
		_, err := svc.List(1)
		assert.Error(t, err)
		db.AssertExpectations(t)
	})
}

// ─── Get ──────────────────────────────────────────────────────────────────────

func TestAddressServiceGet(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		addr := &model.UserAddress{ReceiverName: "张三"}
		db.On("GetByID", uint(1), uint(10)).Return(addr, nil).Once()
		got, err := svc.Get(GetAddressInput{UserID: 1, AddressID: 10})
		assert.NoError(t, err)
		assert.Equal(t, "张三", got.ReceiverName)
		db.AssertExpectations(t)
	})

	t.Run("fail_invalid_user_id", func(t *testing.T) {
		svc, _ := initAddressTestSvc()
		_, err := svc.Get(GetAddressInput{UserID: 0, AddressID: 10})
		assert.Error(t, err)
	})

	t.Run("fail_invalid_address_id", func(t *testing.T) {
		svc, _ := initAddressTestSvc()
		_, err := svc.Get(GetAddressInput{UserID: 1, AddressID: 0})
		assert.Error(t, err)
	})

	t.Run("fail_not_found", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		db.On("GetByID", uint(1), uint(99)).Return(
			(*model.UserAddress)(nil), gorm.ErrRecordNotFound,
		).Once()
		_, err := svc.Get(GetAddressInput{UserID: 1, AddressID: 99})
		assert.Error(t, err)
		db.AssertExpectations(t)
	})

	t.Run("fail_db_error", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		db.On("GetByID", uint(1), uint(10)).Return(
			(*model.UserAddress)(nil), errors.New("db err"),
		).Once()
		_, err := svc.Get(GetAddressInput{UserID: 1, AddressID: 10})
		assert.Error(t, err)
		db.AssertExpectations(t)
	})
}

// ─── Update ───────────────────────────────────────────────────────────────────

func TestAddressServiceUpdate(t *testing.T) {
	existing := &model.UserAddress{ReceiverName: "旧名"}

	t.Run("success", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		inp := UpdateAddressInput{
			UserID:       1,
			AddressID:    10,
			ReceiverName: "新名",
			Phone:        "13900139000",
			Province:     "广东省",
			City:         "广州市",
			District:     "天河区",
			Detail:       "体育西路1号",
		}
		db.On("GetByID", uint(1), uint(10)).Return(existing, nil).Once()
		db.On("Update", matchedAddress(func(a *model.UserAddress) bool {
			return a.ReceiverName == "新名" && a.City == "广州市"
		})).Return(nil).Once()
		assert.NoError(t, svc.Update(inp))
		db.AssertExpectations(t)
	})

	t.Run("success_set_default", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		inp := UpdateAddressInput{
			UserID:       1,
			AddressID:    10,
			ReceiverName: "新名",
			Phone:        "13900139000",
			Province:     "广东省",
			City:         "广州市",
			District:     "天河区",
			Detail:       "体育西路1号",
			IsDefault:    true,
		}
		db.On("GetByID", uint(1), uint(10)).Return(existing, nil).Once()
		db.On("ClearDefault", uint(1)).Return(nil).Once()
		db.On("Update", matchedAddress(func(a *model.UserAddress) bool {
			return a.IsDefault
		})).Return(nil).Once()
		assert.NoError(t, svc.Update(inp))
		db.AssertExpectations(t)
	})

	t.Run("fail_invalid_user_id", func(t *testing.T) {
		svc, _ := initAddressTestSvc()
		assert.Error(t, svc.Update(UpdateAddressInput{UserID: 0, AddressID: 10}))
	})

	t.Run("fail_invalid_address_id", func(t *testing.T) {
		svc, _ := initAddressTestSvc()
		assert.Error(t, svc.Update(UpdateAddressInput{UserID: 1, AddressID: 0}))
	})

	t.Run("fail_missing_field", func(t *testing.T) {
		svc, _ := initAddressTestSvc()
		assert.Error(t, svc.Update(UpdateAddressInput{UserID: 1, AddressID: 10}))
	})

	t.Run("fail_not_found", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		db.On("GetByID", uint(1), uint(99)).Return(
			(*model.UserAddress)(nil), gorm.ErrRecordNotFound,
		).Once()
		err := svc.Update(UpdateAddressInput{
			UserID: 1, AddressID: 99,
			ReceiverName: "x", Phone: "x", Province: "x", City: "x", District: "x", Detail: "x",
		})
		assert.Error(t, err)
		db.AssertExpectations(t)
	})

	t.Run("fail_clear_default_error", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		db.On("GetByID", uint(1), uint(10)).Return(existing, nil).Once()
		db.On("ClearDefault", uint(1)).Return(errors.New("db err")).Once()
		err := svc.Update(UpdateAddressInput{
			UserID: 1, AddressID: 10, IsDefault: true,
			ReceiverName: "x", Phone: "x", Province: "x", City: "x", District: "x", Detail: "x",
		})
		assert.Error(t, err)
		db.AssertExpectations(t)
	})

	t.Run("fail_update_error", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		db.On("GetByID", uint(1), uint(10)).Return(existing, nil).Once()
		db.On("Update", matchedAddress(nil)).Return(errors.New("db err")).Once()
		err := svc.Update(UpdateAddressInput{
			UserID: 1, AddressID: 10,
			ReceiverName: "x", Phone: "x", Province: "x", City: "x", District: "x", Detail: "x",
		})
		assert.Error(t, err)
		db.AssertExpectations(t)
	})
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func TestAddressServiceDelete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		db.On("Delete", uint(1), uint(10)).Return(int64(1), nil).Once()
		assert.NoError(t, svc.Delete(DeleteAddressInput{UserID: 1, AddressID: 10}))
		db.AssertExpectations(t)
	})

	t.Run("fail_invalid_user_id", func(t *testing.T) {
		svc, _ := initAddressTestSvc()
		assert.Error(t, svc.Delete(DeleteAddressInput{UserID: 0, AddressID: 10}))
	})

	t.Run("fail_invalid_address_id", func(t *testing.T) {
		svc, _ := initAddressTestSvc()
		assert.Error(t, svc.Delete(DeleteAddressInput{UserID: 1, AddressID: 0}))
	})

	t.Run("fail_db_error", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		db.On("Delete", uint(1), uint(10)).Return(int64(0), errors.New("db err")).Once()
		assert.Error(t, svc.Delete(DeleteAddressInput{UserID: 1, AddressID: 10}))
		db.AssertExpectations(t)
	})

	t.Run("fail_not_found", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		db.On("Delete", uint(1), uint(99)).Return(int64(0), nil).Once()
		assert.Error(t, svc.Delete(DeleteAddressInput{UserID: 1, AddressID: 99}))
		db.AssertExpectations(t)
	})
}

// ─── SetDefault ───────────────────────────────────────────────────────────────

func TestAddressServiceSetDefault(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		db.On("GetByID", uint(1), uint(10)).Return(
			&model.UserAddress{ReceiverName: "张三"}, nil,
		).Once()
		db.On("ClearDefault", uint(1)).Return(nil).Once()
		db.On("SetDefault", uint(1), uint(10)).Return(int64(1), nil).Once()
		assert.NoError(t, svc.SetDefault(SetDefaultAddressInput{UserID: 1, AddressID: 10}))
		db.AssertExpectations(t)
	})

	t.Run("fail_invalid_user_id", func(t *testing.T) {
		svc, _ := initAddressTestSvc()
		assert.Error(t, svc.SetDefault(SetDefaultAddressInput{UserID: 0, AddressID: 10}))
	})

	t.Run("fail_invalid_address_id", func(t *testing.T) {
		svc, _ := initAddressTestSvc()
		assert.Error(t, svc.SetDefault(SetDefaultAddressInput{UserID: 1, AddressID: 0}))
	})

	t.Run("fail_not_found", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		db.On("GetByID", uint(1), uint(99)).Return(
			(*model.UserAddress)(nil), gorm.ErrRecordNotFound,
		).Once()
		assert.Error(t, svc.SetDefault(SetDefaultAddressInput{UserID: 1, AddressID: 99}))
		db.AssertExpectations(t)
	})

	t.Run("fail_get_by_id_error", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		db.On("GetByID", uint(1), uint(10)).Return(
			(*model.UserAddress)(nil), errors.New("db err"),
		).Once()
		assert.Error(t, svc.SetDefault(SetDefaultAddressInput{UserID: 1, AddressID: 10}))
		db.AssertExpectations(t)
	})

	t.Run("fail_clear_default_error", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		db.On("GetByID", uint(1), uint(10)).Return(
			&model.UserAddress{}, nil,
		).Once()
		db.On("ClearDefault", uint(1)).Return(errors.New("db err")).Once()
		assert.Error(t, svc.SetDefault(SetDefaultAddressInput{UserID: 1, AddressID: 10}))
		db.AssertExpectations(t)
	})

	t.Run("fail_set_default_rows_zero", func(t *testing.T) {
		svc, db := initAddressTestSvc()
		db.On("GetByID", uint(1), uint(10)).Return(
			&model.UserAddress{}, nil,
		).Once()
		db.On("ClearDefault", uint(1)).Return(nil).Once()
		db.On("SetDefault", uint(1), uint(10)).Return(int64(0), nil).Once()
		assert.Error(t, svc.SetDefault(SetDefaultAddressInput{UserID: 1, AddressID: 10}))
		db.AssertExpectations(t)
	})
}

// ─── helper ───────────────────────────────────────────────────────────────────

func matchedAddress(fn func(*model.UserAddress) bool) interface{} {
	return mt.MatchedBy(func(x interface{}) bool {
		if fn == nil {
			return true
		}
		a, ok := x.(*model.UserAddress)
		if !ok {
			return false
		}
		return fn(a)
	})
}
