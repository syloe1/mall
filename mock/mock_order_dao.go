package mock

import (
	"mall/dao"
	"mall/model"

	"github.com/stretchr/testify/mock"
)

// MockOrderDB implements dao.OrderDB using testify/mock.
// For Transaction: by default calls m.Called(fn).Error(0).
// If TxFunc is set, it calls TxFunc(fn) instead — this lets tests
// control whether the transaction callback is actually invoked.
type MockOrderDB struct {
	mock.Mock
	TxFunc func(fn func(txOrderDB dao.OrderDB, txProductDB dao.ProductDB, txCartDB dao.CartDB) error) error
}

func (m *MockOrderDB) Create(order *model.Order) error {
	return m.Called(order).Error(0)
}

func (m *MockOrderDB) GetByID(userID uint, orderNo uint) (*model.Order, error) {
	args := m.Called(userID, orderNo)
	o, ok := args.Get(0).(*model.Order)
	if !ok {
		return nil, args.Error(1)
	}
	return o, args.Error(1)
}

func (m *MockOrderDB) GetByIDForUpdate(userID uint, orderNo uint) (*model.Order, error) {
	args := m.Called(userID, orderNo)
	o, ok := args.Get(0).(*model.Order)
	if !ok {
		return nil, args.Error(1)
	}
	return o, args.Error(1)
}

func (m *MockOrderDB) GetAdminByID(orderID uint) (*model.Order, error) {
	args := m.Called(orderID)
	o, ok := args.Get(0).(*model.Order)
	if !ok {
		return nil, args.Error(1)
	}
	return o, args.Error(1)
}

func (m *MockOrderDB) ListByUser(query dao.OrderListQuery) ([]*model.Order, int64, error) {
	args := m.Called(query)
	list, ok := args.Get(0).([]*model.Order)
	if !ok {
		return nil, 0, args.Error(2)
	}
	return list, args.Get(1).(int64), args.Error(2)
}

func (m *MockOrderDB) ListAdmin(query dao.AdminOrderListQuery) ([]*model.Order, int64, error) {
	args := m.Called(query)
	list, ok := args.Get(0).([]*model.Order)
	if !ok {
		return nil, 0, args.Error(2)
	}
	return list, args.Get(1).(int64), args.Error(2)
}

func (m *MockOrderDB) UpdateStatus(orderID uint, status string) error {
	return m.Called(orderID, status).Error(0)
}

func (m *MockOrderDB) Cancel(userID uint, orderNo uint) (int64, error) {
	args := m.Called(userID, orderNo)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockOrderDB) Transaction(fn func(txOrderDB dao.OrderDB, txProductDB dao.ProductDB, txCartDB dao.CartDB) error) error {
	if m.TxFunc != nil {
		return m.TxFunc(fn)
	}
	return m.Called(fn).Error(0)
}
