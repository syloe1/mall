package mock

import (
	"github.com/stretchr/testify/mock"
)

type MockJwtCfg struct {
	mock.Mock
}

func (m *MockJwtCfg) GetSecret() []byte {
	args := m.Called()
	b, _ := args.Get(0).([]byte)
	return b
}