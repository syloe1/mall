package mock

import (
	"mall/dao"
	"mall/model"

	"github.com/stretchr/testify/mock"
)

type MockPaymentDB struct {
	mock.Mock
}

func (m *MockPaymentDB) Create(payment *model.Payment) error {
	return m.Called(payment).Error(0)
}

func (m *MockPaymentDB) GetByPaymentNo(userID uint, paymentNo string) (*model.Payment, error) {
	args := m.Called(userID, paymentNo)
	p, ok := args.Get(0).(*model.Payment)
	if !ok {
		return nil, args.Error(1)
	}
	return p, args.Error(1)
}

func (m *MockPaymentDB) GetByOrderID(userID uint, orderID uint) (*model.Payment, error) {
	args := m.Called(userID, orderID)
	p, ok := args.Get(0).(*model.Payment)
	if !ok {
		return nil, args.Error(1)
	}
	return p, args.Error(1)
}

func (m *MockPaymentDB) GetByOrderIDForUpdate(userID uint, orderID uint) (*model.Payment, error) {
	args := m.Called(userID, orderID)
	p, ok := args.Get(0).(*model.Payment)
	if !ok {
		return nil, args.Error(1)
	}
	return p, args.Error(1)
}

func (m *MockPaymentDB) GetSuccessByOrderID(userID uint, orderID uint) (*model.Payment, error) {
	args := m.Called(userID, orderID)
	p, ok := args.Get(0).(*model.Payment)
	if !ok {
		return nil, args.Error(1)
	}
	return p, args.Error(1)
}

func (m *MockPaymentDB) ListByUser(query dao.PaymentListQuery) ([]model.Payment, int64, error) {
	args := m.Called(query)
	list, ok := args.Get(0).([]model.Payment)
	if !ok {
		return nil, 0, args.Error(2)
	}
	return list, args.Get(1).(int64), args.Error(2)
}

func (m *MockPaymentDB) Update(payment *model.Payment) error {
	return m.Called(payment).Error(0)
}
