package mock

import (
	"mall/model"

	"github.com/stretchr/testify/mock"
)

type MockAddressDB struct {
	mock.Mock
}

func (m *MockAddressDB) Create(address *model.UserAddress) error {
	return m.Called(address).Error(0)
}

func (m *MockAddressDB) GetByID(userID uint, addressID uint) (*model.UserAddress, error) {
	args := m.Called(userID, addressID)
	a, ok := args.Get(0).(*model.UserAddress)
	if !ok {
		return nil, args.Error(1)
	}
	return a, args.Error(1)
}

func (m *MockAddressDB) GetDefault(userID uint) (*model.UserAddress, error) {
	args := m.Called(userID)
	a, ok := args.Get(0).(*model.UserAddress)
	if !ok {
		return nil, args.Error(1)
	}
	return a, args.Error(1)
}

func (m *MockAddressDB) ListByUserID(userID uint) ([]model.UserAddress, error) {
	args := m.Called(userID)
	list, ok := args.Get(0).([]model.UserAddress)
	if !ok {
		return nil, args.Error(1)
	}
	return list, args.Error(1)
}

func (m *MockAddressDB) CountByUserID(userID uint) (int64, error) {
	args := m.Called(userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockAddressDB) Update(address *model.UserAddress) error {
	return m.Called(address).Error(0)
}

func (m *MockAddressDB) Delete(userID uint, addressID uint) (int64, error) {
	args := m.Called(userID, addressID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockAddressDB) ClearDefault(userID uint) error {
	return m.Called(userID).Error(0)
}

func (m *MockAddressDB) SetDefault(userID uint, addressID uint) (int64, error) {
	args := m.Called(userID, addressID)
	return args.Get(0).(int64), args.Error(1)
}
