package service

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"mall/dao"
	"mall/model"
	"mall/pkg/core"
)

// ═══════════════════════════════════════════════════════════════════════════════
// local mocks
// ═══════════════════════════════════════════════════════════════════════════════

type mockPaymentDB struct {
	// base returns
	payment *model.Payment
	err     error

	// per-method overrides
	getSuccessByOrderIDFunc func(userID, orderID uint) (*model.Payment, error)
	getByOrderIDFunc        func(userID, orderID uint) (*model.Payment, error)
	getByPaymentNoFunc      func(userID uint, paymentNo string) (*model.Payment, error)
	createFunc              func(payment *model.Payment) error
	updateFunc              func(payment *model.Payment) error

	// call recording
	lastCreateArg *model.Payment
	lastUpdateArg *model.Payment
}

func (m *mockPaymentDB) Create(payment *model.Payment) error {
	m.lastCreateArg = payment
	if m.createFunc != nil {
		return m.createFunc(payment)
	}
	return m.err
}
func (m *mockPaymentDB) GetByPaymentNo(userID uint, paymentNo string) (*model.Payment, error) {
	if m.getByPaymentNoFunc != nil {
		return m.getByPaymentNoFunc(userID, paymentNo)
	}
	return m.payment, m.err
}
func (m *mockPaymentDB) GetByOrderID(userID, orderID uint) (*model.Payment, error) {
	if m.getByOrderIDFunc != nil {
		return m.getByOrderIDFunc(userID, orderID)
	}
	return m.payment, m.err
}
func (m *mockPaymentDB) GetByOrderIDForUpdate(userID, orderID uint) (*model.Payment, error) {
	return m.payment, m.err
}
func (m *mockPaymentDB) GetSuccessByOrderID(userID, orderID uint) (*model.Payment, error) {
	if m.getSuccessByOrderIDFunc != nil {
		return m.getSuccessByOrderIDFunc(userID, orderID)
	}
	return m.payment, m.err
}
func (m *mockPaymentDB) ListByUser(query dao.PaymentListQuery) ([]model.Payment, int64, error) {
	return nil, 0, nil
}
func (m *mockPaymentDB) Update(payment *model.Payment) error {
	m.lastUpdateArg = payment
	if m.updateFunc != nil {
		return m.updateFunc(payment)
	}
	return m.err
}

type mockOrderDBForPayment struct {
	order *model.Order
	err   error

	// per-method overrides
	getByIDFunc      func(userID, orderID uint) (*model.Order, error)
	updateStatusFunc func(orderID uint, status string) error

	// call recording
	lastUpdateStatusOrderID uint
	lastUpdateStatusStatus  string
}

func (m *mockOrderDBForPayment) GetByID(userID, orderID uint) (*model.Order, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(userID, orderID)
	}
	return m.order, m.err
}
func (m *mockOrderDBForPayment) UpdateStatus(orderID uint, status string) error {
	m.lastUpdateStatusOrderID = orderID
	m.lastUpdateStatusStatus = status
	if m.updateStatusFunc != nil {
		return m.updateStatusFunc(orderID, status)
	}
	return m.err
}

// stubs
func (m *mockOrderDBForPayment) Create(_ *model.Order) error { return nil }
func (m *mockOrderDBForPayment) GetByIDForUpdate(_, _ uint) (*model.Order, error) {
	return nil, nil
}
func (m *mockOrderDBForPayment) GetAdminByID(_ uint) (*model.Order, error) { return nil, nil }
func (m *mockOrderDBForPayment) ListByUser(_ dao.OrderListQuery) ([]*model.Order, int64, error) {
	return nil, 0, nil
}
func (m *mockOrderDBForPayment) ListAdmin(_ dao.AdminOrderListQuery) ([]*model.Order, int64, error) {
	return nil, 0, nil
}
func (m *mockOrderDBForPayment) Cancel(_, _ uint) (int64, error) { return 0, nil }
func (m *mockOrderDBForPayment) Transaction(_ func(txOrderDB dao.OrderDB, txProductDB dao.ProductDB, txCartDB dao.CartDB) error) error {
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// helpers
// ═══════════════════════════════════════════════════════════════════════════════

func mustBeBizErrorPay(t *testing.T, err error, code int, msgContains string) {
	t.Helper()
	biz, ok := err.(*core.BizError)
	if !ok {
		t.Fatalf("expected *core.BizError, got %T: %v", err, err)
	}
	assert.Equal(t, code, biz.Code, "status code mismatch")
	assert.Contains(t, biz.Msg, msgContains, "message mismatch")
}

// ═══════════════════════════════════════════════════════════════════════════════
// generatePaymentNo / generateMockTransactionNo / generateBalanceTransactionNo
// ═══════════════════════════════════════════════════════════════════════════════

func TestGeneratePaymentNo(t *testing.T) {
	t.Run("format", func(t *testing.T) {
		no := generatePaymentNo(7)
		assert.True(t, strings.HasPrefix(no, "PAY"), "should start with PAY")
		assert.True(t, strings.HasSuffix(no, fmt.Sprintf("%06d", 7)), "should end with 6-digit userID")
		assert.True(t, len(no) > 9, "should be longer than prefix+suffix")
	})
}

func TestGenerateMockTransactionNo(t *testing.T) {
	t.Run("format", func(t *testing.T) {
		no := generateMockTransactionNo(88)
		assert.True(t, strings.HasPrefix(no, "MOCK"))
		assert.True(t, strings.HasSuffix(no, fmt.Sprintf("%06d", 88)))
	})
}

func TestGenerateBalanceTransactionNo(t *testing.T) {
	t.Run("format", func(t *testing.T) {
		no := generateBalanceTransactionNo(123)
		assert.True(t, strings.HasPrefix(no, "BAL"))
		assert.True(t, strings.HasSuffix(no, fmt.Sprintf("%06d", 123)))
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// amountStringToCents
// ═══════════════════════════════════════════════════════════════════════════════

func TestAmountStringToCents(t *testing.T) {

	t.Run("empty_string", func(t *testing.T) {
		cents, err := amountStringToCents("")
		assert.NoError(t, err)
		assert.Equal(t, int64(0), cents)
	})

	t.Run("whitespace_only", func(t *testing.T) {
		cents, err := amountStringToCents("  ")
		assert.NoError(t, err)
		assert.Equal(t, int64(0), cents)
	})

	t.Run("integer_only", func(t *testing.T) {
		cents, err := amountStringToCents("100")
		assert.NoError(t, err)
		assert.Equal(t, int64(10000), cents)
	})

	t.Run("with_cents", func(t *testing.T) {
		cents, err := amountStringToCents("99.50")
		assert.NoError(t, err)
		assert.Equal(t, int64(9950), cents)
	})

	t.Run("single_decimal_padded", func(t *testing.T) {
		cents, err := amountStringToCents("5.9")
		assert.NoError(t, err)
		assert.Equal(t, int64(590), cents)
	})

	t.Run("zero_amount", func(t *testing.T) {
		cents, err := amountStringToCents("0.00")
		assert.NoError(t, err)
		assert.Equal(t, int64(0), cents)
	})

	t.Run("three_decimals_invalid", func(t *testing.T) {
		_, err := amountStringToCents("1.999")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid amount precision")
	})

	t.Run("two_dots_invalid", func(t *testing.T) {
		_, err := amountStringToCents("1.2.3")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid amount")
	})

	t.Run("non_number_invalid", func(t *testing.T) {
		_, err := amountStringToCents("abc")
		assert.Error(t, err)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// centsToAmountString
// ═══════════════════════════════════════════════════════════════════════════════

func TestCentsToAmountString(t *testing.T) {

	t.Run("zero", func(t *testing.T) {
		assert.Equal(t, "0.00", centsToAmountString(0))
	})

	t.Run("with_cents", func(t *testing.T) {
		assert.Equal(t, "99.50", centsToAmountString(9950))
	})

	t.Run("integer_yuan", func(t *testing.T) {
		assert.Equal(t, "100.00", centsToAmountString(10000))
	})

	t.Run("single_digit_cents", func(t *testing.T) {
		assert.Equal(t, "0.05", centsToAmountString(5))
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// GetPayment
// ═══════════════════════════════════════════════════════════════════════════════

func TestPaymentService_GetPayment(t *testing.T) {
	makeSvc := func(db dao.PaymentDB) *PaymentService {
		return &PaymentService{paymentDB: db}
	}

	t.Run("invalid_user_id", func(t *testing.T) {
		svc := makeSvc(&mockPaymentDB{})
		p, err := svc.GetPayment(GetPaymentInput{UserID: 0, PaymentNo: "PAY123"})
		assert.Nil(t, p)
		mustBeBizErrorPay(t, err, http.StatusBadRequest, "用户 ID 不合法")
	})

	t.Run("empty_payment_no", func(t *testing.T) {
		svc := makeSvc(&mockPaymentDB{})
		p, err := svc.GetPayment(GetPaymentInput{UserID: 1, PaymentNo: ""})
		assert.Nil(t, p)
		mustBeBizErrorPay(t, err, http.StatusBadRequest, "支付单号不合法")
	})

	t.Run("whitespace_payment_no", func(t *testing.T) {
		svc := makeSvc(&mockPaymentDB{})
		p, err := svc.GetPayment(GetPaymentInput{UserID: 1, PaymentNo: "   "})
		assert.Nil(t, p)
		mustBeBizErrorPay(t, err, http.StatusBadRequest, "支付单号不合法")
	})

	t.Run("payment_not_found", func(t *testing.T) {
		db := &mockPaymentDB{err: gorm.ErrRecordNotFound}
		svc := makeSvc(db)
		p, err := svc.GetPayment(GetPaymentInput{UserID: 1, PaymentNo: "PAY999"})
		assert.Nil(t, p)
		mustBeBizErrorPay(t, err, http.StatusNotFound, "支付单不存在")
	})

	t.Run("db_error", func(t *testing.T) {
		dbErr := errors.New("db down")
		db := &mockPaymentDB{err: dbErr}
		svc := makeSvc(db)
		p, err := svc.GetPayment(GetPaymentInput{UserID: 1, PaymentNo: "PAY001"})
		assert.Nil(t, p)
		assert.Equal(t, dbErr, err)
	})

	t.Run("success", func(t *testing.T) {
		want := &model.Payment{PaymentNo: "PAY001"}
		db := &mockPaymentDB{payment: want}
		svc := makeSvc(db)
		p, err := svc.GetPayment(GetPaymentInput{UserID: 1, PaymentNo: "PAY001"})
		assert.NoError(t, err)
		assert.Equal(t, want, p)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// withPaymentLock (nil rdb)
// ═══════════════════════════════════════════════════════════════════════════════

func TestPaymentService_WithPaymentLock_NilRDB(t *testing.T) {
	t.Run("nil_rdb_passthrough", func(t *testing.T) {
		svc := &PaymentService{rdb: nil}
		wantPayment := &model.Payment{PaymentNo: "PAY_OK"}

		called := false
		result, err := svc.withPaymentLock("some-key", func() (*model.Payment, error) {
			called = true
			return wantPayment, nil
		})

		assert.NoError(t, err)
		assert.True(t, called, "fn should have been invoked")
		assert.Equal(t, wantPayment, result)
	})

	t.Run("nil_rdb_passthrough_error", func(t *testing.T) {
		svc := &PaymentService{rdb: nil}
		fnErr := errors.New("fn failed")

		result, err := svc.withPaymentLock("key", func() (*model.Payment, error) {
			return nil, fnErr
		})

		assert.Equal(t, fnErr, err)
		assert.Nil(t, result)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// mockPaySuccessLocked
// ═══════════════════════════════════════════════════════════════════════════════

func TestPaymentService_MockPaySuccessLocked(t *testing.T) {
	validOrder := &model.Order{
		Status:      model.OrderPendingPay,
		TotalAmount: 5000,
	}
	validOrder.ID = 10

	// helper: fresh pending payment each time (method mutates it)
	newValidPayment := func() *model.Payment {
		return &model.Payment{
			PaymentNo: "PAY001",
			OrderID:   10,
			Amount:    5000,
			Status:    model.PaymentStatusPending,
			Channel:   model.PaymentChannelMock,
		}
	}

	t.Run("already_success", func(t *testing.T) {
		svc := &PaymentService{}
		p := &model.Payment{Status: model.PaymentStatusSuccess}
		result, err := svc.mockPaySuccessLocked(1, p)
		assert.NoError(t, err)
		assert.Equal(t, p, result)
	})

	t.Run("not_pending_status", func(t *testing.T) {
		svc := &PaymentService{}
		p := &model.Payment{Status: model.PaymentStatusFailed}
		result, err := svc.mockPaySuccessLocked(1, p)
		assert.Nil(t, result)
		mustBeBizErrorPay(t, err, http.StatusBadRequest, "支付单当前状态不可支付")
	})

	t.Run("order_not_found", func(t *testing.T) {
		orderDB := &mockOrderDBForPayment{err: gorm.ErrRecordNotFound}
		svc := &PaymentService{orderDB: orderDB}
		result, err := svc.mockPaySuccessLocked(1, newValidPayment())
		assert.Nil(t, result)
		mustBeBizErrorPay(t, err, http.StatusNotFound, "订单不存在")
	})

	t.Run("order_db_error", func(t *testing.T) {
		dbErr := errors.New("db error")
		orderDB := &mockOrderDBForPayment{err: dbErr}
		svc := &PaymentService{orderDB: orderDB}
		result, err := svc.mockPaySuccessLocked(1, newValidPayment())
		assert.Nil(t, result)
		assert.Equal(t, dbErr, err)
	})

	t.Run("order_already_paid", func(t *testing.T) {
		paidOrder := &model.Order{Status: model.OrderPaid, TotalAmount: 5000}
		paidOrder.ID = 10
		orderDB := &mockOrderDBForPayment{order: paidOrder}
		paymentDB := &mockPaymentDB{}
		svc := &PaymentService{orderDB: orderDB, paymentDB: paymentDB}

		result, err := svc.mockPaySuccessLocked(1, newValidPayment())
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, model.PaymentStatusSuccess, result.Status)
		assert.NotNil(t, result.PaidAt)
		assert.NotEmpty(t, result.TransactionNo)
		// 验证 Update 被调用
		assert.NotNil(t, paymentDB.lastUpdateArg)
		assert.Equal(t, model.PaymentStatusSuccess, paymentDB.lastUpdateArg.Status)
	})

	t.Run("order_not_pending_pay", func(t *testing.T) {
		orderDB := &mockOrderDBForPayment{order: &model.Order{Status: model.OrderCanceled}}
		svc := &PaymentService{orderDB: orderDB}
		result, err := svc.mockPaySuccessLocked(1, newValidPayment())
		assert.Nil(t, result)
		mustBeBizErrorPay(t, err, http.StatusBadRequest, "订单当前状态不可支付")
	})

	t.Run("amount_mismatch", func(t *testing.T) {
		mismatchOrder := &model.Order{Status: model.OrderPendingPay, TotalAmount: 9999}
		mismatchOrder.ID = 10
		orderDB := &mockOrderDBForPayment{order: mismatchOrder}
		svc := &PaymentService{orderDB: orderDB}
		result, err := svc.mockPaySuccessLocked(1, newValidPayment())
		assert.Nil(t, result)
		mustBeBizErrorPay(t, err, http.StatusBadRequest, "支付金额不一致")
	})

	t.Run("success", func(t *testing.T) {
		orderDB := &mockOrderDBForPayment{order: validOrder}
		paymentDB := &mockPaymentDB{}
		svc := &PaymentService{orderDB: orderDB, paymentDB: paymentDB}

		result, err := svc.mockPaySuccessLocked(1, newValidPayment())
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, model.PaymentStatusSuccess, result.Status)
		assert.NotNil(t, result.PaidAt)
		assert.True(t, strings.HasPrefix(result.TransactionNo, "MOCK"))

		// 验证 payment.Update 被调用
		assert.NotNil(t, paymentDB.lastUpdateArg)

		// 验证 order.UpdateStatus 被调用
		assert.Equal(t, uint(10), orderDB.lastUpdateStatusOrderID)
		assert.Equal(t, model.OrderPaid, orderDB.lastUpdateStatusStatus)
	})

	t.Run("payment_update_error", func(t *testing.T) {
		orderDB := &mockOrderDBForPayment{order: validOrder}
		updateErr := errors.New("update payment failed")
		paymentDB := &mockPaymentDB{updateFunc: func(p *model.Payment) error { return updateErr }}
		svc := &PaymentService{orderDB: orderDB, paymentDB: paymentDB}

		result, err := svc.mockPaySuccessLocked(1, newValidPayment())
		assert.Nil(t, result)
		assert.Equal(t, updateErr, err)
	})

	t.Run("order_update_status_error", func(t *testing.T) {
		orderDB := &mockOrderDBForPayment{order: validOrder}
		updateStatusErr := errors.New("update order failed")
		orderDB.updateStatusFunc = func(orderID uint, status string) error { return updateStatusErr }
		paymentDB := &mockPaymentDB{}
		svc := &PaymentService{orderDB: orderDB, paymentDB: paymentDB}

		result, err := svc.mockPaySuccessLocked(1, newValidPayment())
		assert.Nil(t, result)
		assert.Equal(t, updateStatusErr, err)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// CreatePayment (mock channel, rdb=nil)
// ═══════════════════════════════════════════════════════════════════════════════

func TestPaymentService_CreatePayment_MockChannel(t *testing.T) {

	t.Run("invalid_user_id", func(t *testing.T) {
		svc := &PaymentService{rdb: nil}
		p, err := svc.CreatePayment(CreatePaymentInput{UserID: 0, OrderID: 1, Channel: model.PaymentChannelMock})
		assert.Nil(t, p)
		mustBeBizErrorPay(t, err, http.StatusBadRequest, "用户 ID 不合法")
	})

	t.Run("invalid_order_id", func(t *testing.T) {
		svc := &PaymentService{rdb: nil}
		p, err := svc.CreatePayment(CreatePaymentInput{UserID: 1, OrderID: 0, Channel: model.PaymentChannelMock})
		assert.Nil(t, p)
		mustBeBizErrorPay(t, err, http.StatusBadRequest, "订单 ID 不合法")
	})

	t.Run("empty_channel", func(t *testing.T) {
		svc := &PaymentService{rdb: nil}
		p, err := svc.CreatePayment(CreatePaymentInput{UserID: 1, OrderID: 1, Channel: ""})
		assert.Nil(t, p)
		mustBeBizErrorPay(t, err, http.StatusBadRequest, "支付渠道不合法")
	})

	t.Run("unsupported_channel", func(t *testing.T) {
		svc := &PaymentService{rdb: nil}
		p, err := svc.CreatePayment(CreatePaymentInput{UserID: 1, OrderID: 1, Channel: "wechat"})
		assert.Nil(t, p)
		mustBeBizErrorPay(t, err, http.StatusBadRequest, "暂不支持该支付渠道")
	})

	t.Run("order_not_found", func(t *testing.T) {
		orderDB := &mockOrderDBForPayment{err: gorm.ErrRecordNotFound}
		svc := &PaymentService{rdb: nil, orderDB: orderDB}
		p, err := svc.CreatePayment(CreatePaymentInput{UserID: 1, OrderID: 99, Channel: model.PaymentChannelMock})
		assert.Nil(t, p)
		mustBeBizErrorPay(t, err, http.StatusNotFound, "订单不存在")
	})

	t.Run("order_db_error", func(t *testing.T) {
		dbErr := errors.New("db timeout")
		orderDB := &mockOrderDBForPayment{err: dbErr}
		svc := &PaymentService{rdb: nil, orderDB: orderDB}
		p, err := svc.CreatePayment(CreatePaymentInput{UserID: 1, OrderID: 1, Channel: model.PaymentChannelMock})
		assert.Nil(t, p)
		assert.Equal(t, dbErr, err)
	})

	t.Run("order_already_paid", func(t *testing.T) {
		paidOrder := &model.Order{Status: model.OrderPaid, TotalAmount: 1000}
		paidOrder.ID = 1
		orderDB := &mockOrderDBForPayment{order: paidOrder}
		svc := &PaymentService{rdb: nil, orderDB: orderDB}
		p, err := svc.CreatePayment(CreatePaymentInput{UserID: 1, OrderID: 1, Channel: model.PaymentChannelMock})
		assert.Nil(t, p)
		mustBeBizErrorPay(t, err, http.StatusBadRequest, "订单已支付")
	})

	t.Run("order_not_pending_pay", func(t *testing.T) {
		canceledOrder := &model.Order{Status: model.OrderCanceled, TotalAmount: 1000}
		canceledOrder.ID = 1
		orderDB := &mockOrderDBForPayment{order: canceledOrder}
		svc := &PaymentService{rdb: nil, orderDB: orderDB}
		p, err := svc.CreatePayment(CreatePaymentInput{UserID: 1, OrderID: 1, Channel: model.PaymentChannelMock})
		assert.Nil(t, p)
		mustBeBizErrorPay(t, err, http.StatusBadRequest, "订单当前状态不可支付")
	})

	t.Run("invalid_amount_zero", func(t *testing.T) {
		zeroOrder := &model.Order{Status: model.OrderPendingPay, TotalAmount: 0}
		zeroOrder.ID = 1
		orderDB := &mockOrderDBForPayment{order: zeroOrder}
		svc := &PaymentService{rdb: nil, orderDB: orderDB}
		p, err := svc.CreatePayment(CreatePaymentInput{UserID: 1, OrderID: 1, Channel: model.PaymentChannelMock})
		assert.Nil(t, p)
		mustBeBizErrorPay(t, err, http.StatusBadRequest, "订单金额不合法")
	})

	t.Run("get_success_payment_db_error", func(t *testing.T) {
		validOrder := &model.Order{Status: model.OrderPendingPay, TotalAmount: 5000}
		validOrder.ID = 1
		orderDB := &mockOrderDBForPayment{order: validOrder}
		dbErr := errors.New("db error")
		paymentDB := &mockPaymentDB{err: dbErr} // GetSuccessByOrderID fails
		svc := &PaymentService{rdb: nil, orderDB: orderDB, paymentDB: paymentDB}
		p, err := svc.CreatePayment(CreatePaymentInput{UserID: 1, OrderID: 1, Channel: model.PaymentChannelMock})
		assert.Nil(t, p)
		assert.Equal(t, dbErr, err)
	})

	t.Run("already_has_success_payment", func(t *testing.T) {
		validOrder := &model.Order{Status: model.OrderPendingPay, TotalAmount: 5000}
		validOrder.ID = 1
		paidOrderNo := "PAY100001"
		validOrder.OrderNo = paidOrderNo
		orderDB := &mockOrderDBForPayment{order: validOrder}
		existPayment := &model.Payment{PaymentNo: "PAY_DONE", Status: model.PaymentStatusSuccess}
		// GetSuccessByOrderID returns success payment
		paymentDB := &mockPaymentDB{payment: existPayment}
		svc := &PaymentService{rdb: nil, orderDB: orderDB, paymentDB: paymentDB}
		p, err := svc.CreatePayment(CreatePaymentInput{UserID: 1, OrderID: 1, Channel: model.PaymentChannelMock})
		assert.NoError(t, err)
		assert.Equal(t, existPayment, p)
	})

	t.Run("has_pending_payment_returns_existing", func(t *testing.T) {
		validOrder := &model.Order{Status: model.OrderPendingPay, TotalAmount: 5000}
		validOrder.ID = 1
		validOrder.OrderNo = "PAY100001"
		orderDB := &mockOrderDBForPayment{order: validOrder}
		pendingPayment := &model.Payment{
			PaymentNo: "PAY_PENDING",
			Status:    model.PaymentStatusPending,
			Channel:   model.PaymentChannelMock,
			Amount:    5000,
		}
		paymentDB := &mockPaymentDB{
			// GetSuccessByOrderID → record not found
			err: gorm.ErrRecordNotFound,
			// GetByOrderID → returns pending payment
			getByOrderIDFunc: func(userID, orderID uint) (*model.Payment, error) {
				return pendingPayment, nil
			},
		}
		svc := &PaymentService{rdb: nil, orderDB: orderDB, paymentDB: paymentDB}
		p, err := svc.CreatePayment(CreatePaymentInput{UserID: 1, OrderID: 1, Channel: model.PaymentChannelMock})
		assert.NoError(t, err)
		assert.Equal(t, pendingPayment, p)
	})

	t.Run("get_by_order_id_db_error", func(t *testing.T) {
		validOrder := &model.Order{Status: model.OrderPendingPay, TotalAmount: 5000}
		validOrder.ID = 1
		orderDB := &mockOrderDBForPayment{order: validOrder}
		dbErr := errors.New("db error")
		paymentDB := &mockPaymentDB{
			// GetSuccessByOrderID → not found
			getSuccessByOrderIDFunc: func(userID, orderID uint) (*model.Payment, error) {
				return nil, gorm.ErrRecordNotFound
			},
			// GetByOrderID → db error
			getByOrderIDFunc: func(userID, orderID uint) (*model.Payment, error) {
				return nil, dbErr
			},
		}
		svc := &PaymentService{rdb: nil, orderDB: orderDB, paymentDB: paymentDB}
		p, err := svc.CreatePayment(CreatePaymentInput{UserID: 1, OrderID: 1, Channel: model.PaymentChannelMock})
		assert.Nil(t, p)
		assert.Equal(t, dbErr, err)
	})

	t.Run("create_new_payment_success", func(t *testing.T) {
		validOrder := &model.Order{Status: model.OrderPendingPay, TotalAmount: 5000}
		validOrder.ID = 1
		validOrder.OrderNo = "PAY100001"
		orderDB := &mockOrderDBForPayment{order: validOrder}
		paymentDB := &mockPaymentDB{
			// GetSuccessByOrderID → not found
			getSuccessByOrderIDFunc: func(userID, orderID uint) (*model.Payment, error) {
				return nil, gorm.ErrRecordNotFound
			},
			// GetByOrderID → not found
			getByOrderIDFunc: func(userID, orderID uint) (*model.Payment, error) {
				return nil, gorm.ErrRecordNotFound
			},
		}
		svc := &PaymentService{rdb: nil, orderDB: orderDB, paymentDB: paymentDB}
		p, err := svc.CreatePayment(CreatePaymentInput{UserID: 7, OrderID: 1, Channel: model.PaymentChannelMock})
		assert.NoError(t, err)
		assert.NotNil(t, p)
		assert.Equal(t, uint(7), p.UserID)
		assert.Equal(t, uint(1), p.OrderID)
		assert.Equal(t, validOrder.OrderNo, p.OrderNo)
		assert.Equal(t, model.PaymentChannelMock, p.Channel)
		assert.Equal(t, int64(5000), p.Amount)
		assert.Equal(t, model.PaymentStatusPending, p.Status)
		assert.True(t, strings.HasPrefix(p.PaymentNo, "PAY"))

		// 验证 Create 被调用
		assert.NotNil(t, paymentDB.lastCreateArg)
		assert.Equal(t, model.PaymentChannelMock, paymentDB.lastCreateArg.Channel)
	})

	t.Run("create_payment_db_error", func(t *testing.T) {
		validOrder := &model.Order{Status: model.OrderPendingPay, TotalAmount: 5000}
		validOrder.ID = 1
		validOrder.OrderNo = "PAY100001"
		orderDB := &mockOrderDBForPayment{order: validOrder}
		createErr := errors.New("create payment failed")
		paymentDB := &mockPaymentDB{
			getSuccessByOrderIDFunc: func(userID, orderID uint) (*model.Payment, error) {
				return nil, gorm.ErrRecordNotFound
			},
			getByOrderIDFunc: func(userID, orderID uint) (*model.Payment, error) {
				return nil, gorm.ErrRecordNotFound
			},
			createFunc: func(p *model.Payment) error {
				return createErr
			},
		}
		svc := &PaymentService{rdb: nil, orderDB: orderDB, paymentDB: paymentDB}
		p, err := svc.CreatePayment(CreatePaymentInput{UserID: 1, OrderID: 1, Channel: model.PaymentChannelMock})
		assert.Nil(t, p)
		assert.Equal(t, createErr, err)
	})
}
