package mock

import (
	"mall/model"

	"github.com/stretchr/testify/mock"
)

type MockProductOutboxDB struct {
	mock.Mock
}

func (m *MockProductOutboxDB) Create(event *model.ProductOutbox) error {
	return m.Called(event).Error(0)
}

func (m *MockProductOutboxDB) ListPending(limit int) ([]model.ProductOutbox, error) {
	args := m.Called(limit)
	list, ok := args.Get(0).([]model.ProductOutbox)
	if !ok {
		return nil, args.Error(1)
	}
	return list, args.Error(1)
}

func (m *MockProductOutboxDB) MarkPublished(id uint) error {
	return m.Called(id).Error(0)
}

func (m *MockProductOutboxDB) MarkFailed(id uint, errMsg string) error {
	return m.Called(id, errMsg).Error(0)
}
