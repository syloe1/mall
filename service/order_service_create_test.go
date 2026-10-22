package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"mall/dao"
	"mall/model"
)

// ═══════════════════════════════════════════════════════════════════════════════
// inline mocks — tx DAOs used inside Transaction callbacks
// ═══════════════════════════════════════════════════════════════════════════════

type mockTxOrderDB struct {
	createErr   error
	lastCreated *model.Order
}

func (m *mockTxOrderDB) Create(order *model.Order) error { m.lastCreated = order; return m.createErr }
func (m *mockTxOrderDB) GetByID(_, _ uint) (*model.Order, error)                  { return nil, nil }
func (m *mockTxOrderDB) GetByIDForUpdate(_, _ uint) (*model.Order, error)         { return nil, nil }
func (m *mockTxOrderDB) GetAdminByID(_ uint) (*model.Order, error)                { return nil, nil }
func (m *mockTxOrderDB) ListByUser(_ dao.OrderListQuery) ([]*model.Order, int64, error)      { return nil, 0, nil }
func (m *mockTxOrderDB) ListAdmin(_ dao.AdminOrderListQuery) ([]*model.Order, int64, error)   { return nil, 0, nil }
func (m *mockTxOrderDB) UpdateStatus(_ uint, _ string) error                                    { return nil }
func (m *mockTxOrderDB) Cancel(_, _ uint) (int64, error)                                        { return 0, nil }
func (m *mockTxOrderDB) Transaction(_ func(txOrderDB dao.OrderDB, txProductDB dao.ProductDB, txCartDB dao.CartDB) error) error {
	return nil
}

type mockTxCartDB struct {
	checkedItems []model.CartItem
	listErr      error
	deleteErr    error
}

func (m *mockTxCartDB) ListCheckedItems(_ uint) ([]model.CartItem, error)       { return m.checkedItems, m.listErr }
func (m *mockTxCartDB) DeleteItem(_, _ uint) (int64, error)                     { return 1, m.deleteErr }
func (m *mockTxCartDB) GetItem(_, _ uint) (*model.CartItem, error)              { return nil, nil }
func (m *mockTxCartDB) CreateItem(_ *model.CartItem) error                      { return nil }
func (m *mockTxCartDB) UpdateItem(_ *model.CartItem) error                      { return nil }
func (m *mockTxCartDB) Clear(_ uint) error                                      { return nil }
func (m *mockTxCartDB) ListItems(_ uint) ([]model.CartItem, error)              { return nil, nil }
func (m *mockTxCartDB) SetItemChecked(_, _ uint, _ bool) (int64, error)         { return 0, nil }
func (m *mockTxCartDB) SetAllChecked(_ uint, _ bool) error                      { return nil }

type mockTxProductDB struct {
	product         *model.Product
	getErr          error
	decreaseStockOk bool
	decreaseErr     error
}

func (m *mockTxProductDB) GetProductById(_ uint) (*model.Product, error) {
	return m.product, m.getErr
}
func (m *mockTxProductDB) DecreaseStock(_ uint, _ int) (int64, error) {
	if m.decreaseErr != nil { return 0, m.decreaseErr }
	if m.decreaseStockOk { return 1, nil }
	return 0, nil
}
func (m *mockTxProductDB) Create(_ *model.Product) error                                      { return nil }
func (m *mockTxProductDB) GetPublicByID(_ uint) (*model.Product, error)                        { return nil, nil }
func (m *mockTxProductDB) ListAdmin(_ dao.ProductListQuery) ([]model.Product, int64, error)     { return nil, 0, nil }
func (m *mockTxProductDB) ListPublic(_ dao.ProductListQuery) ([]model.Product, int64, error)    { return nil, 0, nil }
func (m *mockTxProductDB) Update(_ *model.Product) error                                        { return nil }
func (m *mockTxProductDB) AddImage(_ *model.ProductImage) error                                 { return nil }
func (m *mockTxProductDB) DeleteImage(_, _ uint) (int64, error)                                 { return 0, nil }
func (m *mockTxProductDB) DeleteImagesByProductID(_ uint) error                                 { return nil }
func (m *mockTxProductDB) DeleteProductByID(_ uint) error                                       { return nil }
func (m *mockTxProductDB) SearchPublic(_ dao.ProductListQuery) ([]model.Product, int64, error)  { return nil, 0, nil }
func (m *mockTxProductDB) Transaction(_ func(txProductDB dao.ProductDB, txOutboxDB dao.ProductOutboxDB) error) error {
	return nil
}

// fakeOrderDB lets us fully control Transaction behavior.
type fakeOrderDB struct {
	txFn func(fn func(txOrderDB dao.OrderDB, txProductDB dao.ProductDB, txCartDB dao.CartDB) error) error
}

func (f *fakeOrderDB) Transaction(fn func(txOrderDB dao.OrderDB, txProductDB dao.ProductDB, txCartDB dao.CartDB) error) error {
	return f.txFn(fn)
}
func (f *fakeOrderDB) Create(_ *model.Order) error                 { return nil }
func (f *fakeOrderDB) GetByID(_, _ uint) (*model.Order, error)     { return nil, nil }
func (f *fakeOrderDB) GetByIDForUpdate(_, _ uint) (*model.Order, error) { return nil, nil }
func (f *fakeOrderDB) GetAdminByID(_ uint) (*model.Order, error)   { return nil, nil }
func (f *fakeOrderDB) ListByUser(_ dao.OrderListQuery) ([]*model.Order, int64, error)       { return nil, 0, nil }
func (f *fakeOrderDB) ListAdmin(_ dao.AdminOrderListQuery) ([]*model.Order, int64, error)    { return nil, 0, nil }
func (f *fakeOrderDB) UpdateStatus(_ uint, _ string) error          { return nil }
func (f *fakeOrderDB) Cancel(_, _ uint) (int64, error)              { return 0, nil }

// ═══════════════════════════════════════════════════════════════════════════════
// helpers
// ═══════════════════════════════════════════════════════════════════════════════

func validOrderAddress() *model.UserAddress {
	return &model.UserAddress{
		ReceiverName: "张三", Phone: "13800138000",
		Province: "广东", City: "深圳", District: "南山",
		Detail: "科技园", PostalCode: "518000",
	}
}

func onSaleProductForOrder() *model.Product {
	p := &model.Product{Name: "测试商品", PriceCents: 9900, Stock: 100, Status: model.ProductOnSale,
		Images: []model.ProductImage{{URL: "cover.jpg"}}}
	p.ID = 1
	return p
}

// ═══════════════════════════════════════════════════════════════════════════════
// CreateFromCart
// ═══════════════════════════════════════════════════════════════════════════════

func TestOrderService_CreateFromCart(t *testing.T) {

	t.Run("invalid_user_id", func(t *testing.T) {
		svc := &OrderService{}
		order, err := svc.CreateFromCart(CreateOrderFromCartInput{UserID: 0, AddressID: 1})
		assert.Nil(t, order)
		mustBeBizErrorOrder(t, err, http.StatusBadRequest, "用户 ID 不合法")
	})

	t.Run("invalid_address_id", func(t *testing.T) {
		svc := &OrderService{}
		order, err := svc.CreateFromCart(CreateOrderFromCartInput{UserID: 1, AddressID: 0})
		assert.Nil(t, order)
		mustBeBizErrorOrder(t, err, http.StatusBadRequest, "地址 ID 不合法")
	})

	t.Run("address_not_found", func(t *testing.T) {
		addrDB := &mockAddressDBForOrder{err: gorm.ErrRecordNotFound}
		svc := &OrderService{orderDB: &fakeOrderDB{}, addressDB: addrDB, producer: nil}
		order, err := svc.CreateFromCart(CreateOrderFromCartInput{UserID: 1, AddressID: 1})
		assert.Nil(t, order)
		mustBeBizErrorOrder(t, err, http.StatusNotFound, "收货地址不存在")
	})

	t.Run("cart_empty", func(t *testing.T) {
		addrDB := &mockAddressDBForOrder{address: validOrderAddress()}
		customTx := func(fn func(txOrderDB dao.OrderDB, txProductDB dao.ProductDB, txCartDB dao.CartDB) error) error {
			return fn(&mockTxOrderDB{}, &mockTxProductDB{}, &mockTxCartDB{}) // checkedItems nil → empty
		}
		svc := &OrderService{orderDB: &fakeOrderDB{txFn: customTx}, addressDB: addrDB, producer: nil}
		order, err := svc.CreateFromCart(CreateOrderFromCartInput{UserID: 1, AddressID: 1})
		assert.Nil(t, order)
		mustBeBizErrorOrder(t, err, http.StatusBadRequest, "请先勾选要结算的商品")
	})

	t.Run("product_not_found_in_build", func(t *testing.T) {
		addrDB := &mockAddressDBForOrder{address: validOrderAddress()}
		customTx := func(fn func(txOrderDB dao.OrderDB, txProductDB dao.ProductDB, txCartDB dao.CartDB) error) error {
			txCart := &mockTxCartDB{checkedItems: []model.CartItem{{ProductID: 99, Quantity: 1}}}
			txProd := &mockTxProductDB{getErr: gorm.ErrRecordNotFound}
			return fn(&mockTxOrderDB{}, txProd, txCart)
		}
		svc := &OrderService{orderDB: &fakeOrderDB{txFn: customTx}, addressDB: addrDB, producer: nil}
		order, err := svc.CreateFromCart(CreateOrderFromCartInput{UserID: 1, AddressID: 1})
		assert.Nil(t, order)
		mustBeBizErrorOrder(t, err, http.StatusNotFound, "商品不存在或已下架")
	})

	t.Run("decrease_stock_fails_rows_zero", func(t *testing.T) {
		addrDB := &mockAddressDBForOrder{address: validOrderAddress()}
		customTx := func(fn func(txOrderDB dao.OrderDB, txProductDB dao.ProductDB, txCartDB dao.CartDB) error) error {
			txCart := &mockTxCartDB{checkedItems: []model.CartItem{{ProductID: 1, Quantity: 5}}}
			txProd := &mockTxProductDB{product: onSaleProductForOrder(), decreaseStockOk: false}
			return fn(&mockTxOrderDB{}, txProd, txCart)
		}
		svc := &OrderService{orderDB: &fakeOrderDB{txFn: customTx}, addressDB: addrDB, producer: nil}
		order, err := svc.CreateFromCart(CreateOrderFromCartInput{UserID: 1, AddressID: 1})
		assert.Nil(t, order)
		mustBeBizErrorOrder(t, err, http.StatusBadRequest, "库存不足")
	})

	t.Run("success", func(t *testing.T) {
		addrDB := &mockAddressDBForOrder{address: validOrderAddress()}
		customTx := func(fn func(txOrderDB dao.OrderDB, txProductDB dao.ProductDB, txCartDB dao.CartDB) error) error {
			txCart := &mockTxCartDB{checkedItems: []model.CartItem{{UserID: 1, ProductID: 1, Quantity: 2}}}
			txProd := &mockTxProductDB{product: onSaleProductForOrder(), decreaseStockOk: true}
			return fn(&mockTxOrderDB{}, txProd, txCart)
		}
		svc := &OrderService{orderDB: &fakeOrderDB{txFn: customTx}, addressDB: addrDB, producer: nil}
		order, err := svc.CreateFromCart(CreateOrderFromCartInput{UserID: 1, AddressID: 1})
		assert.NoError(t, err)
		assert.NotNil(t, order)
		assert.Equal(t, model.OrderPendingPay, order.Status)
		assert.Len(t, order.Items, 1)
		assert.Equal(t, "张三", order.ReceiverName)
		assert.Equal(t, int64(9900*2), order.TotalAmount)
	})

	t.Run("transaction_error_propagated", func(t *testing.T) {
		addrDB := &mockAddressDBForOrder{address: validOrderAddress()}
		dbErr := errors.New("cart list error")
		customTx := func(fn func(txOrderDB dao.OrderDB, txProductDB dao.ProductDB, txCartDB dao.CartDB) error) error {
			txCart := &mockTxCartDB{listErr: dbErr}
			return fn(&mockTxOrderDB{}, &mockTxProductDB{}, txCart)
		}
		svc := &OrderService{orderDB: &fakeOrderDB{txFn: customTx}, addressDB: addrDB, producer: nil}
		order, err := svc.CreateFromCart(CreateOrderFromCartInput{UserID: 1, AddressID: 1})
		assert.Nil(t, order)
		assert.Equal(t, dbErr, err)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// BuyNow
// ═══════════════════════════════════════════════════════════════════════════════

func TestOrderService_BuyNow(t *testing.T) {

	t.Run("invalid_user_id", func(t *testing.T) {
		svc := &OrderService{}
		order, err := svc.BuyNow(BuyNowInput{UserID: 0, ProductID: 1, Quantity: 1, AddressID: 1})
		assert.Nil(t, order)
		mustBeBizErrorOrder(t, err, http.StatusBadRequest, "用户 ID 不合法")
	})

	t.Run("invalid_product_id", func(t *testing.T) {
		svc := &OrderService{}
		order, err := svc.BuyNow(BuyNowInput{UserID: 1, ProductID: 0, Quantity: 1, AddressID: 1})
		assert.Nil(t, order)
		mustBeBizErrorOrder(t, err, http.StatusBadRequest, "商品 ID 不合法")
	})

	t.Run("invalid_quantity", func(t *testing.T) {
		svc := &OrderService{}
		order, err := svc.BuyNow(BuyNowInput{UserID: 1, ProductID: 1, Quantity: 0, AddressID: 1})
		assert.Nil(t, order)
		mustBeBizErrorOrder(t, err, http.StatusBadRequest, "数量必须大于 0")
	})

	t.Run("invalid_address_id", func(t *testing.T) {
		svc := &OrderService{}
		order, err := svc.BuyNow(BuyNowInput{UserID: 1, ProductID: 1, Quantity: 1, AddressID: 0})
		assert.Nil(t, order)
		mustBeBizErrorOrder(t, err, http.StatusBadRequest, "地址 ID 不合法")
	})

	t.Run("address_not_found", func(t *testing.T) {
		addrDB := &mockAddressDBForOrder{err: gorm.ErrRecordNotFound}
		svc := &OrderService{orderDB: &fakeOrderDB{}, addressDB: addrDB, producer: nil}
		order, err := svc.BuyNow(BuyNowInput{UserID: 1, ProductID: 1, Quantity: 1, AddressID: 1})
		assert.Nil(t, order)
		mustBeBizErrorOrder(t, err, http.StatusNotFound, "收货地址不存在")
	})

	t.Run("product_not_found", func(t *testing.T) {
		addrDB := &mockAddressDBForOrder{address: validOrderAddress()}
		customTx := func(fn func(txOrderDB dao.OrderDB, txProductDB dao.ProductDB, txCartDB dao.CartDB) error) error {
			return fn(&mockTxOrderDB{}, &mockTxProductDB{getErr: gorm.ErrRecordNotFound}, &mockTxCartDB{})
		}
		svc := &OrderService{orderDB: &fakeOrderDB{txFn: customTx}, addressDB: addrDB, producer: nil}
		order, err := svc.BuyNow(BuyNowInput{UserID: 1, ProductID: 99, Quantity: 1, AddressID: 1})
		assert.Nil(t, order)
		mustBeBizErrorOrder(t, err, http.StatusNotFound, "商品不存在或已下架")
	})

	t.Run("success", func(t *testing.T) {
		addrDB := &mockAddressDBForOrder{address: validOrderAddress()}
		customTx := func(fn func(txOrderDB dao.OrderDB, txProductDB dao.ProductDB, txCartDB dao.CartDB) error) error {
			return fn(&mockTxOrderDB{}, &mockTxProductDB{product: onSaleProductForOrder(), decreaseStockOk: true}, &mockTxCartDB{})
		}
		svc := &OrderService{orderDB: &fakeOrderDB{txFn: customTx}, addressDB: addrDB, producer: nil}
		order, err := svc.BuyNow(BuyNowInput{UserID: 1, ProductID: 1, Quantity: 3, AddressID: 1})
		assert.NoError(t, err)
		assert.NotNil(t, order)
		assert.Len(t, order.Items, 1)
		assert.Equal(t, int64(9900*3), order.TotalAmount)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// createOrderWithLock — with Redis (miniredis)
// ═══════════════════════════════════════════════════════════════════════════════

func newMiniRedis(t *testing.T) (*redis.Client, func()) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return client, func() { client.Close(); mr.Close() }
}

func TestOrderService_CreateOrderWithLock(t *testing.T) {

	t.Run("nil_redis_passthrough_success", func(t *testing.T) {
		svc := &OrderService{rdb: nil}
		order := &model.Order{OrderNo: "ORD-001"}
		got, err := svc.createOrderWithLock(1, func() (*model.Order, error) { return order, nil })
		assert.NoError(t, err)
		assert.Equal(t, order, got)
	})

	t.Run("nil_redis_passthrough_error", func(t *testing.T) {
		svc := &OrderService{rdb: nil}
		fnErr := errors.New("fn error")
		got, err := svc.createOrderWithLock(1, func() (*model.Order, error) { return nil, fnErr })
		assert.Nil(t, got)
		assert.Equal(t, fnErr, err)
	})

	t.Run("lock_acquired_success", func(t *testing.T) {
		rdb, cleanup := newMiniRedis(t)
		defer cleanup()
		svc := &OrderService{rdb: rdb}
		order := &model.Order{OrderNo: "ORD-002"}
		got, err := svc.createOrderWithLock(1, func() (*model.Order, error) { return order, nil })
		assert.NoError(t, err)
		assert.Equal(t, order, got)
	})

	t.Run("lock_not_acquired_duplicate_request", func(t *testing.T) {
		rdb, cleanup := newMiniRedis(t)
		defer cleanup()
		svc := &OrderService{rdb: rdb}
		got1, err1 := svc.createOrderWithLock(1, func() (*model.Order, error) {
			got2, err2 := svc.createOrderWithLock(1, func() (*model.Order, error) { return &model.Order{}, nil })
			assert.Nil(t, got2)
			mustBeBizErrorOrder(t, err2, http.StatusTooManyRequests, "订单提交过于频繁，请稍后再试")
			return &model.Order{OrderNo: "ORD-003"}, nil
		})
		assert.NoError(t, err1)
		assert.NotNil(t, got1)
	})

	t.Run("different_users_can_acquire", func(t *testing.T) {
		rdb, cleanup := newMiniRedis(t)
		defer cleanup()
		svc := &OrderService{rdb: rdb}
		got1, err1 := svc.createOrderWithLock(1, func() (*model.Order, error) { return &model.Order{OrderNo: "U1"}, nil })
		require.NoError(t, err1)
		got2, err2 := svc.createOrderWithLock(2, func() (*model.Order, error) { return &model.Order{OrderNo: "U2"}, nil })
		assert.NoError(t, err2)
		assert.Equal(t, "U1", got1.OrderNo)
		assert.Equal(t, "U2", got2.OrderNo)
	})
}
