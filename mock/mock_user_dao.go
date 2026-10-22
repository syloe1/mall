package mock

import (
	"mall/dao"
	"mall/model"

	"github.com/stretchr/testify/mock"
)

type MockUserDB struct {
	mock.Mock
	// TxFunc, if set, is used by Transaction instead of m.Called(fn).
	// This lets tests actually invoke the transaction callback when needed.
	TxFunc func(fn func(tx dao.UserDB) error) error
}

func (m *MockUserDB) GetByUsername(username string) (*model.User, error) {
	args := m.Called(username)
	u, ok := args.Get(0).(*model.User)
	if !ok {
		return nil, args.Error(1)
	}
	return u, args.Error(1)
}

func (m *MockUserDB) FindById(uid uint) (*model.User, error) {
	args := m.Called(uid)
	u, ok := args.Get(0).(*model.User)
	if !ok {
		return nil, args.Error(1)
	}
	return u, args.Error(1)
}

func (m *MockUserDB) FindByIDForUpdate(uid uint) (*model.User, error) {
	args := m.Called(uid)
	u, ok := args.Get(0).(*model.User)
	if !ok {
		return nil, args.Error(1)
	}
	return u, args.Error(1)
}

func (m *MockUserDB) Create(u *model.User) error {
	return m.Called(u).Error(0)
}

func (m *MockUserDB) Update(u *model.User) error {
	return m.Called(u).Error(0)
}

func (m *MockUserDB) Transaction(fn func(tx dao.UserDB) error) error {
	if m.TxFunc != nil {
		return m.TxFunc(fn)
	}
	return m.Called(fn).Error(0)
}
