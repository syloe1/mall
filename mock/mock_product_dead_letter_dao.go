package mock

import (
	"mall/dao"
	"mall/model"

	"github.com/stretchr/testify/mock"
)

type MockProductDeadLetterDB struct {
	mock.Mock
}

func (m *MockProductDeadLetterDB) Create(record *model.ProductEventDeadLetter) error {
	return m.Called(record).Error(0)
}

func (m *MockProductDeadLetterDB) List(query dao.ProductDeadLetterQuery) ([]model.ProductEventDeadLetter, int64, error) {
	args := m.Called(query)
	list, ok := args.Get(0).([]model.ProductEventDeadLetter)
	if !ok {
		return nil, 0, args.Error(2)
	}
	return list, args.Get(1).(int64), args.Error(2)
}
