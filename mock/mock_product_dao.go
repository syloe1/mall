package mock

import (
	"mall/dao"
	"mall/model"

	"github.com/stretchr/testify/mock"
)

// MockProductDB implements dao.ProductDB using testify/mock.
// For Transaction: by default calls m.Called(fn).Error(0).
// If TxFunc is set, it calls TxFunc(fn) instead.
type MockProductDB struct {
	mock.Mock
	TxFunc func(fn func(txProductDB dao.ProductDB, txOutboxDB dao.ProductOutboxDB) error) error
}

func (m *MockProductDB) Create(product *model.Product) error {
	return m.Called(product).Error(0)
}

func (m *MockProductDB) GetProductById(id uint) (*model.Product, error) {
	args := m.Called(id)
	p, ok := args.Get(0).(*model.Product)
	if !ok {
		return nil, args.Error(1)
	}
	return p, args.Error(1)
}

func (m *MockProductDB) GetPublicByID(id uint) (*model.Product, error) {
	args := m.Called(id)
	p, ok := args.Get(0).(*model.Product)
	if !ok {
		return nil, args.Error(1)
	}
	return p, args.Error(1)
}

func (m *MockProductDB) ListAdmin(query dao.ProductListQuery) ([]model.Product, int64, error) {
	args := m.Called(query)
	list, ok := args.Get(0).([]model.Product)
	if !ok {
		return nil, 0, args.Error(2)
	}
	return list, args.Get(1).(int64), args.Error(2)
}

func (m *MockProductDB) ListPublic(query dao.ProductListQuery) ([]model.Product, int64, error) {
	args := m.Called(query)
	list, ok := args.Get(0).([]model.Product)
	if !ok {
		return nil, 0, args.Error(2)
	}
	return list, args.Get(1).(int64), args.Error(2)
}

func (m *MockProductDB) Update(product *model.Product) error {
	return m.Called(product).Error(0)
}

func (m *MockProductDB) AddImage(image *model.ProductImage) error {
	return m.Called(image).Error(0)
}

func (m *MockProductDB) DeleteImage(productID uint, imageID uint) (int64, error) {
	args := m.Called(productID, imageID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockProductDB) DeleteImagesByProductID(productID uint) error {
	return m.Called(productID).Error(0)
}

func (m *MockProductDB) DeleteProductByID(productID uint) error {
	return m.Called(productID).Error(0)
}

func (m *MockProductDB) SearchPublic(query dao.ProductListQuery) ([]model.Product, int64, error) {
	args := m.Called(query)
	list, ok := args.Get(0).([]model.Product)
	if !ok {
		return nil, 0, args.Error(2)
	}
	return list, args.Get(1).(int64), args.Error(2)
}

func (m *MockProductDB) DecreaseStock(productId uint, quantity int) (int64, error) {
	args := m.Called(productId, quantity)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockProductDB) Transaction(fn func(txProductDB dao.ProductDB, txOutboxDB dao.ProductOutboxDB) error) error {
	if m.TxFunc != nil {
		return m.TxFunc(fn)
	}
	return m.Called(fn).Error(0)
}
