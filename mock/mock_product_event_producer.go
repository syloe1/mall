package mock

import (
	"context"
	"mall/pkg/mq"

	"github.com/stretchr/testify/mock"
)

type MockProductEventProducer struct {
	mock.Mock
}

func (m *MockProductEventProducer) PublishProductEvent(ctx context.Context, event mq.ProductEvent) error {
	return m.Called(ctx, event).Error(0)
}
