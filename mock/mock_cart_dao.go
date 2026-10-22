package mock

import (
	"mall/model"

	"github.com/stretchr/testify/mock"
)

type MockCartDB struct {
	mock.Mock
}

func (m *MockCartDB) GetItem(userID uint, productID uint) (*model.CartItem, error) {
	args := m.Called(userID, productID)
	item, ok := args.Get(0).(*model.CartItem)
	if !ok {
		return nil, args.Error(1)
	}
	return item, args.Error(1)
}

func (m *MockCartDB) CreateItem(item *model.CartItem) error {
	return m.Called(item).Error(0)
}

func (m *MockCartDB) UpdateItem(item *model.CartItem) error {
	return m.Called(item).Error(0)
}

func (m *MockCartDB) DeleteItem(userID uint, productID uint) (int64, error) {
	args := m.Called(userID, productID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockCartDB) Clear(userID uint) error {
	return m.Called(userID).Error(0)
}

func (m *MockCartDB) ListItems(userID uint) ([]model.CartItem, error) {
	args := m.Called(userID)
	list, ok := args.Get(0).([]model.CartItem)
	if !ok {
		return nil, args.Error(1)
	}
	return list, args.Error(1)
}

func (m *MockCartDB) ListCheckedItems(userID uint) ([]model.CartItem, error) {
	args := m.Called(userID)
	list, ok := args.Get(0).([]model.CartItem)
	if !ok {
		return nil, args.Error(1)
	}
	return list, args.Error(1)
}

func (m *MockCartDB) SetItemChecked(userID uint, productID uint, checked bool) (int64, error) {
	args := m.Called(userID, productID, checked)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockCartDB) SetAllChecked(userID uint, checked bool) error {
	return m.Called(userID, checked).Error(0)
}
