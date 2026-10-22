package mock

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/mock"
)

type MockRedisClient struct {
	mock.Mock
}

func (m *MockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	args := m.Called(ctx, key)
	return args.Get(0).(*redis.StringCmd)
}
func (m *MockRedisClient) Set(ctx context.Context, key string, val interface{}, exp time.Duration) *redis.StatusCmd {
	args := m.Called(ctx, key, val, exp)
	return args.Get(0).(*redis.StatusCmd)
}