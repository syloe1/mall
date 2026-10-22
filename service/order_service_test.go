package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"mall/dao"
	"mall/model"
	"mall/pkg/core"
	"mall/pkg/mq"
)

// ═══════════════════════════════════════════════════════════════════════════════
// local mocks (minimal — only the methods actually called by the tested code)
// ═══════════════════════════════════════════════════════════════════════════════

// --- AddressDB mock (for getAddress) ---

type mockAddressDBForOrder struct {
	address *model.UserAddress
	err     error
}

func (m *mockAddressDBForOrder) GetByID(userID, addressID uint) (*model.UserAddress, error) {
	return m.address, m.err
}

func (m *mockAddressDBForOrder) GetDefault(userID uint) (*model.UserAddress, error) {
	return m.address, m.err
}

// stubs
func (m *mockAddressDBForOrder) Create(_ *model.UserAddress) error                       { return nil }
func (m *mockAddressDBForOrder) ListByUserID(_ uint) ([]model.UserAddress, error)         { return nil, nil }
func (m *mockAddressDBForOrder) CountByUserID(_ uint) (int64, error)                      { return 0, nil }
func (m *mockAddressDBForOrder) Update(_ *model.UserAddress) error                        { return nil }
func (m *mockAddressDBForOrder) Delete(_, _ uint) (int64, error)                          { return 0, nil }
func (m *mockAddressDBForOrder) ClearDefault(_ uint) error                                { return nil }
func (m *mockAddressDBForOrder) SetDefault(_, _ uint) (int64, error)                      { return 0, nil }

// --- OrderDB mock (for CancelTimeoutOrder) ---

type mockOrderDBForCancel struct {
	rowsAffected int64
	err          error
}

func (m *mockOrderDBForCancel) Cancel(userID, orderNo uint) (int64, error) {
	return m.rowsAffected, m.err
}

// stubs
func (m *mockOrderDBForCancel) Create(_ *model.Order) error                                 { return nil }
func (m *mockOrderDBForCancel) GetByID(_, _ uint) (*model.Order, error)                    { return nil, nil }
func (m *mockOrderDBForCancel) GetByIDForUpdate(_, _ uint) (*model.Order, error)           { return nil, nil }
func (m *mockOrderDBForCancel) GetAdminByID(_ uint) (*model.Order, error)                  { return nil, nil }
func (m *mockOrderDBForCancel) ListByUser(_ dao.OrderListQuery) ([]*model.Order, int64, error) {
	return nil, 0, nil
}
func (m *mockOrderDBForCancel) ListAdmin(_ dao.AdminOrderListQuery) ([]*model.Order, int64, error) {
	return nil, 0, nil
}
func (m *mockOrderDBForCancel) UpdateStatus(_ uint, _ string) error                        { return nil }
func (m *mockOrderDBForCancel) Transaction(_ func(txOrderDB dao.OrderDB, txProductDB dao.ProductDB, txCartDB dao.CartDB) error) error {
	return nil
}

// --- ProductDB mock (for buildOrder) ---

type mockProductDBForOrder struct {
	product *model.Product
	err     error
}

func (m *mockProductDBForOrder) GetProductById(id uint) (*model.Product, error) {
	return m.product, m.err
}

// stubs
func (m *mockProductDBForOrder) Create(_ *model.Product) error                            { return nil }
func (m *mockProductDBForOrder) GetPublicByID(_ uint) (*model.Product, error)             { return nil, nil }
func (m *mockProductDBForOrder) ListAdmin(_ dao.ProductListQuery) ([]model.Product, int64, error) {
	return nil, 0, nil
}
func (m *mockProductDBForOrder) ListPublic(_ dao.ProductListQuery) ([]model.Product, int64, error) {
	return nil, 0, nil
}
func (m *mockProductDBForOrder) Update(_ *model.Product) error                             { return nil }
func (m *mockProductDBForOrder) AddImage(_ *model.ProductImage) error                      { return nil }
func (m *mockProductDBForOrder) DeleteImage(_, _ uint) (int64, error)                      { return 0, nil }
func (m *mockProductDBForOrder) DeleteImagesByProductID(_ uint) error                      { return nil }
func (m *mockProductDBForOrder) DeleteAllImages(_ uint) (int64, error)                     { return 0, nil }
func (m *mockProductDBForOrder) DeleteProductByID(_ uint) error                            { return nil }
func (m *mockProductDBForOrder) SearchPublic(_ dao.ProductListQuery) ([]model.Product, int64, error) {
	return nil, 0, nil
}
func (m *mockProductDBForOrder) DecreaseStock(_ uint, _ int) (int64, error)                { return 1, nil }
func (m *mockProductDBForOrder) Transaction(_ func(txProductDB dao.ProductDB, txOutboxDB dao.ProductOutboxDB) error) error {
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// helpers
// ═══════════════════════════════════════════════════════════════════════════════

func mustBeBizErrorOrder(t *testing.T, err error, code int, msgContains string) {
	t.Helper()
	biz, ok := err.(*core.BizError)
	if !ok {
		t.Fatalf("expected *core.BizError, got %T: %v", err, err)
	}
	assert.Equal(t, code, biz.Code, "status code mismatch")
	assert.Contains(t, biz.Msg, msgContains, "message mismatch")
}

// ═══════════════════════════════════════════════════════════════════════════════
// firstProductImage
// ═══════════════════════════════════════════════════════════════════════════════

func TestFirstProductImage(t *testing.T) {

	t.Run("nil_images", func(t *testing.T) {
		p := &model.Product{Images: nil}
		assert.Equal(t, "", firstProductImage(p))
	})

	t.Run("empty_images", func(t *testing.T) {
		p := &model.Product{Images: []model.ProductImage{}}
		assert.Equal(t, "", firstProductImage(p))
	})

	t.Run("single_image", func(t *testing.T) {
		p := &model.Product{Images: []model.ProductImage{
			{URL: "https://img.example.com/a.jpg"},
		}}
		assert.Equal(t, "https://img.example.com/a.jpg", firstProductImage(p))
	})

	t.Run("multiple_images_returns_first", func(t *testing.T) {
		p := &model.Product{Images: []model.ProductImage{
			{URL: "first.jpg"},
			{URL: "second.jpg"},
		}}
		assert.Equal(t, "first.jpg", firstProductImage(p))
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// generateOrderNo
// ═══════════════════════════════════════════════════════════════════════════════

func TestGenerateOrderNo(t *testing.T) {
	t.Run("format", func(t *testing.T) {
		no := generateOrderNo(42)
		// 格式: <nanos>000042（6 位 userID）
		assert.True(t, len(no) > 6, "order no should be longer than 6 chars")
		assert.True(t, strings.HasSuffix(no, fmt.Sprintf("%06d", 42)),
			"order no should end with zero-padded userID")

		// 大 userID 也不被截断（最后 6 位完整）
		no2 := generateOrderNo(123456)
		assert.True(t, strings.HasSuffix(no2, "123456"), "6-digit userID should appear as-is")
	})

	t.Run("different_nanos_produce_different_prefix", func(t *testing.T) {
		no1 := generateOrderNo(1)
		no2 := generateOrderNo(1)
		// 同一个纳秒内可能相同，但大概率不同；这里只验证格式一致
		assert.True(t, strings.HasSuffix(no1, fmt.Sprintf("%06d", 1)))
		assert.True(t, strings.HasSuffix(no2, fmt.Sprintf("%06d", 1)))
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// getAddress
// ═══════════════════════════════════════════════════════════════════════════════

func TestOrderService_GetAddress(t *testing.T) {
	makeSvc := func(db dao.AddressDB) *OrderService {
		return &OrderService{addressDB: db}
	}

	t.Run("success", func(t *testing.T) {
		mock := &mockAddressDBForOrder{
			address: &model.UserAddress{
				ReceiverName: "张三",
				Phone:        "13800138000",
				Province:     "广东",
				City:         "深圳",
				District:     "南山",
				Detail:       "科技园",
				PostalCode:   "518000",
			},
		}
		svc := makeSvc(mock)
		addr, err := svc.getAddress(1, 10)
		assert.NoError(t, err)
		assert.Equal(t, "张三", addr.ReceiverName)
	})

	t.Run("address_not_found", func(t *testing.T) {
		mock := &mockAddressDBForOrder{err: gorm.ErrRecordNotFound}
		svc := makeSvc(mock)
		addr, err := svc.getAddress(1, 99)
		assert.Nil(t, addr)
		mustBeBizErrorOrder(t, err, http.StatusNotFound, "收货地址不存在")
	})

	t.Run("db_error", func(t *testing.T) {
		dbErr := errors.New("db timeout")
		mock := &mockAddressDBForOrder{err: dbErr}
		svc := makeSvc(mock)
		addr, err := svc.getAddress(1, 1)
		assert.Nil(t, addr)
		assert.Equal(t, dbErr, err)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// CancelTimeoutOrder
// ═══════════════════════════════════════════════════════════════════════════════

func TestOrderService_CancelTimeoutOrder(t *testing.T) {
	makeSvc := func(db dao.OrderDB) *OrderService {
		return &OrderService{orderDB: db}
	}

	t.Run("success", func(t *testing.T) {
		mock := &mockOrderDBForCancel{rowsAffected: 1}
		svc := makeSvc(mock)
		err := svc.CancelTimeoutOrder(1, 100)
		assert.NoError(t, err)
	})

	t.Run("db_error", func(t *testing.T) {
		dbErr := errors.New("cancel failed")
		mock := &mockOrderDBForCancel{err: dbErr}
		svc := makeSvc(mock)
		err := svc.CancelTimeoutOrder(1, 100)
		assert.Equal(t, dbErr, err)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// publishOrderTimeout
// ═══════════════════════════════════════════════════════════════════════════════

// --- OrderTimeoutProducer fake (for publishOrderTimeout) ---

// fakeOrderTimeoutProducer 记录实际发出的延迟消息，用于断言「非 nil producer 时消息真的发出去了」。
// 这个 fake 能存在，是因为 producer 字段被抽成了 OrderTimeoutProducer 接口。
type fakeOrderTimeoutProducer struct {
	calls      int
	routingKey string
	msg        mq.Message
	delay      time.Duration
	err        error
}

func (f *fakeOrderTimeoutProducer) PublishDelayJSON(
	_ context.Context, routingKey string, v any, _ string, delay time.Duration,
) error {
	f.calls++
	f.routingKey = routingKey
	f.delay = delay
	if m, ok := v.(mq.Message); ok {
		f.msg = m
	}
	return f.err
}

func TestOrderService_PublishOrderTimeout(t *testing.T) {

	t.Run("nil_producer_noop", func(t *testing.T) {
		svc := &OrderService{producer: nil}
		// 没注入生产者时是 no-op（会打 Warn 日志），不能 panic
		svc.publishOrderTimeout(&model.Order{UserID: 1})
	})

	// 下面两个用例覆盖「有 producer」的分支 —— 之前 producer 是具体类型 *mq.Producer，
	// 注入不了 fake，这条路径完全没有测试。
	t.Run("publishes_delay_message", func(t *testing.T) {
		producer := &fakeOrderTimeoutProducer{}
		svc := &OrderService{producer: producer}

		order := &model.Order{UserID: 7}
		order.ID = 42

		svc.publishOrderTimeout(order)

		assert.Equal(t, 1, producer.calls, "延迟关单消息必须发出，否则订单永远不会被超时取消")
		assert.Equal(t, mq.OrderDelayRoutingKey, producer.routingKey)
		assert.Equal(t, 30*time.Minute, producer.delay, "延迟时长必须是 30 分钟")
		assert.Equal(t, "order.created", producer.msg.Event)
		assert.Equal(t, uint(42), producer.msg.BizID, "BizID 必须是订单 ID")
		assert.Equal(t, uint(7), producer.msg.UserID)
		assert.NotEmpty(t, producer.msg.MsgID)
	})

	// 发消息失败绝不向上传播：调用时订单已经落库，若让调用方以为「建单失败」，
	// 就会触发库存回滚，而订单行还在 → 孤儿订单 + 库存被重复卖出（超卖）。
	t.Run("publish_error_is_swallowed", func(t *testing.T) {
		producer := &fakeOrderTimeoutProducer{err: errors.New("rabbitmq down")}
		svc := &OrderService{producer: producer}

		svc.publishOrderTimeout(&model.Order{UserID: 1})

		assert.Equal(t, 1, producer.calls, "失败也要确认确实尝试过发布")
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// buildOrder
// ═══════════════════════════════════════════════════════════════════════════════

func TestOrderService_BuildOrder(t *testing.T) {
	// buildOrder doesn't use any s.* field (productDB is a parameter),
	// so a zero-value OrderService is fine.
	svc := &OrderService{}

	validAddress := &model.UserAddress{
		ReceiverName: "李四",
		Phone:        "13900139000",
		Province:     "北京",
		City:         "北京",
		District:     "朝阳",
		Detail:       "望京",
		PostalCode:   "100000",
	}

	onSaleProduct := &model.Product{
		Name:       "测试商品",
		PriceCents: 9900,
		Stock:      100,
		Status:     model.ProductOnSale,
		Images: []model.ProductImage{
			{URL: "cover.jpg"},
		},
	}
	onSaleProduct.ID = 1 // gorm.Model ID — used by buildOrder as OrderItem.ProductID

	t.Run("product_not_found", func(t *testing.T) {
		productDB := &mockProductDBForOrder{err: gorm.ErrRecordNotFound}
		order, err := svc.buildOrder(1, validAddress, []orderBuildItem{
			{ProductID: 999, Quantity: 1},
		}, productDB)
		assert.Nil(t, order)
		mustBeBizErrorOrder(t, err, http.StatusNotFound, "商品不存在或已下架")
	})

	t.Run("get_product_db_error", func(t *testing.T) {
		dbErr := errors.New("db error")
		productDB := &mockProductDBForOrder{err: dbErr}
		order, err := svc.buildOrder(1, validAddress, []orderBuildItem{
			{ProductID: 1, Quantity: 1},
		}, productDB)
		assert.Nil(t, order)
		assert.Equal(t, dbErr, err)
	})

	t.Run("product_not_on_sale", func(t *testing.T) {
		p := &model.Product{Name: "下架商品", PriceCents: 100, Stock: 10, Status: model.ProductOffSale}
		productDB := &mockProductDBForOrder{product: p}
		order, err := svc.buildOrder(1, validAddress, []orderBuildItem{
			{ProductID: 1, Quantity: 1},
		}, productDB)
		assert.Nil(t, order)
		mustBeBizErrorOrder(t, err, http.StatusBadRequest, "商品已下架")
	})

	t.Run("insufficient_stock", func(t *testing.T) {
		p := &model.Product{Name: "库存不足", PriceCents: 100, Stock: 2, Status: model.ProductOnSale}
		productDB := &mockProductDBForOrder{product: p}
		order, err := svc.buildOrder(1, validAddress, []orderBuildItem{
			{ProductID: 1, Quantity: 5},
		}, productDB)
		assert.Nil(t, order)
		mustBeBizErrorOrder(t, err, http.StatusBadRequest, "库存不足")
	})

	t.Run("success_single_item", func(t *testing.T) {
		productDB := &mockProductDBForOrder{product: onSaleProduct}
		order, err := svc.buildOrder(7, validAddress, []orderBuildItem{
			{ProductID: 1, Quantity: 3},
		}, productDB)
		assert.NoError(t, err)
		assert.NotNil(t, order)

		// 订单基本信息
		assert.Equal(t, uint(7), order.UserID)
		assert.Equal(t, model.OrderPendingPay, order.Status)
		assert.NotEmpty(t, order.OrderNo)

		// 地址复制
		assert.Equal(t, "李四", order.ReceiverName)
		assert.Equal(t, "13900139000", order.Phone)
		assert.Equal(t, "北京", order.Province)
		assert.Equal(t, "北京", order.City)
		assert.Equal(t, "朝阳", order.District)
		assert.Equal(t, "望京", order.Detail)
		assert.Equal(t, "100000", order.PostalCode)

		// 金额
		assert.Equal(t, int64(9900*3), order.TotalAmount)

		// item
		assert.Len(t, order.Items, 1)
		item := order.Items[0]
		assert.Equal(t, uint(1), item.ProductID)
		assert.Equal(t, "测试商品", item.ProductName)
		assert.Equal(t, "cover.jpg", item.ProductImage)
		assert.Equal(t, int64(9900), item.PriceCents)
		assert.Equal(t, 3, item.Quantity)
		assert.Equal(t, int64(9900*3), item.Subtotal)
	})

	t.Run("success_multiple_items", func(t *testing.T) {
		productDB := &mockProductDBForOrder{product: onSaleProduct}
		order, err := svc.buildOrder(1, validAddress, []orderBuildItem{
			{ProductID: 1, Quantity: 2},
			{ProductID: 2, Quantity: 5},
		}, productDB)
		assert.NoError(t, err)
		assert.Len(t, order.Items, 2)
		// total = 9900*2 + 9900*5 = 9900*7
		assert.Equal(t, int64(9900*7), order.TotalAmount)
	})

	t.Run("product_without_image", func(t *testing.T) {
		p := &model.Product{Name: "无图", PriceCents: 500, Stock: 10, Status: model.ProductOnSale}
		productDB := &mockProductDBForOrder{product: p}
		order, err := svc.buildOrder(1, validAddress, []orderBuildItem{
			{ProductID: 1, Quantity: 1},
		}, productDB)
		assert.NoError(t, err)
		assert.Equal(t, "", order.Items[0].ProductImage)
	})
}
