package mock

import (
	"mall/model"

	"github.com/stretchr/testify/mock"
)

type MockSeckillDB struct {
	mock.Mock
}

func (m *MockSeckillDB) GetActivityByID(activityID uint) (*model.SeckillActivity, error) {
	args := m.Called(activityID)
	a, ok := args.Get(0).(*model.SeckillActivity)
	if !ok {
		return nil, args.Error(1)
	}
	return a, args.Error(1)
}

func (m *MockSeckillDB) CreatePurchase(purchase *model.SeckillPurchase) error {
	return m.Called(purchase).Error(0)
}

func (m *MockSeckillDB) ListActiveActivities() ([]*model.SeckillActivity, error) {
	args := m.Called()
	a, _ := args.Get(0).([]*model.SeckillActivity)
	return a, args.Error(1)
}

func (m *MockSeckillDB) GetPurchase(userID, activityID uint) (*model.SeckillPurchase, error) {
	args := m.Called(userID, activityID)
	p, _ := args.Get(0).(*model.SeckillPurchase)
	return p, args.Error(1)
}
