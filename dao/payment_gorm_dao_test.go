package dao

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"mall/model"
)

// ─── helpers ────────────────────────────────────────────────────────────────────

func initTestPaymentDB(t *testing.T) *GormPaymentDB {
	t.Helper()
	db := newTestDB(t, &model.Payment{}, &model.Order{})
	return &GormPaymentDB{DB: db}
}

func seedPayment(t *testing.T, db *gorm.DB, opts ...func(*model.Payment)) *model.Payment {
	t.Helper()
	p := &model.Payment{
		PaymentNo: "PAY" + randomSuffix(),
		OrderID:   1,
		OrderNo:   "ORD-001",
		UserID:    1,
		Channel:   model.PaymentChannelMock,
		Amount:    9900,
		Status:    model.PaymentStatusPending,
	}
	for _, opt := range opts {
		opt(p)
	}
	require.NoError(t, db.Create(p).Error)
	return p
}

func seedOrderForPayment(t *testing.T, db *gorm.DB, opts ...func(*model.Order)) *model.Order {
	t.Helper()
	o := &model.Order{
		OrderNo:     "ORD-001",
		UserID:      1,
		Status:      model.OrderPendingPay,
		TotalAmount: 9900,
	}
	for _, opt := range opts {
		opt(o)
	}
	require.NoError(t, db.Create(o).Error)
	return o
}

// ─── Create ─────────────────────────────────────────────────────────────────────

func TestGormPaymentDB_Create(t *testing.T) {
	g := initTestPaymentDB(t)
	p := &model.Payment{PaymentNo: "PAY-001", OrderID: 1, UserID: 1, Channel: model.PaymentChannelMock, Amount: 9900, Status: model.PaymentStatusPending}
	assert.NoError(t, g.Create(p))
	assert.NotZero(t, p.ID)
}

// ─── GetByPaymentNo ─────────────────────────────────────────────────────────────

func TestGormPaymentDB_GetByPaymentNo(t *testing.T) {
	g := initTestPaymentDB(t)
	seed := seedPayment(t, g.DB)

	got, err := g.GetByPaymentNo(seed.UserID, seed.PaymentNo)
	assert.NoError(t, err)
	assert.Equal(t, seed.PaymentNo, got.PaymentNo)
}

func TestGormPaymentDB_GetByPaymentNo_NotFound(t *testing.T) {
	g := initTestPaymentDB(t)
	_, err := g.GetByPaymentNo(1, "NONEXISTENT")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ─── GetByOrderID ───────────────────────────────────────────────────────────────

func TestGormPaymentDB_GetByOrderID(t *testing.T) {
	g := initTestPaymentDB(t)
	_ = seedOrderForPayment(t, g.DB)
	seed := seedPayment(t, g.DB, func(p *model.Payment) { p.OrderID = 1 })

	got, err := g.GetByOrderID(seed.UserID, seed.OrderID)
	assert.NoError(t, err)
	assert.Equal(t, seed.ID, got.ID)
}

func TestGormPaymentDB_GetByOrderID_Latest(t *testing.T) {
	// Payment.OrderID has uniqueIndex, so there can only be one payment per order.
	// Verify the method returns the correct (only) payment for the order.
	g := initTestPaymentDB(t)
	seedOrderForPayment(t, g.DB, func(o *model.Order) { o.ID = 10 })
	seed := seedPayment(t, g.DB, func(p *model.Payment) { p.OrderID = 10; p.PaymentNo = "PAY-ONLY" })

	got, err := g.GetByOrderID(seed.UserID, seed.OrderID)
	assert.NoError(t, err)
	assert.Equal(t, "PAY-ONLY", got.PaymentNo)
}

func TestGormPaymentDB_GetByOrderID_NotFound(t *testing.T) {
	g := initTestPaymentDB(t)
	_, err := g.GetByOrderID(1, 9999)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ─── GetByOrderIDForUpdate ──────────────────────────────────────────────────────

func TestGormPaymentDB_GetByOrderIDForUpdate(t *testing.T) {
	g := initTestPaymentDB(t)
	_ = seedOrderForPayment(t, g.DB)
	seed := seedPayment(t, g.DB, func(p *model.Payment) { p.OrderID = 1 })

	got, err := g.GetByOrderIDForUpdate(seed.UserID, seed.OrderID)
	assert.NoError(t, err)
	assert.Equal(t, seed.ID, got.ID)
}

func TestGormPaymentDB_GetByOrderIDForUpdate_NotFound(t *testing.T) {
	g := initTestPaymentDB(t)
	_, err := g.GetByOrderIDForUpdate(1, 9999)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ─── GetSuccessByOrderID ────────────────────────────────────────────────────────

func TestGormPaymentDB_GetSuccessByOrderID(t *testing.T) {
	g := initTestPaymentDB(t)
	_ = seedOrderForPayment(t, g.DB)
	seed := seedPayment(t, g.DB, func(p *model.Payment) { p.OrderID = 1; p.Status = model.PaymentStatusSuccess })

	got, err := g.GetSuccessByOrderID(seed.UserID, seed.OrderID)
	assert.NoError(t, err)
	assert.Equal(t, model.PaymentStatusSuccess, got.Status)
}

func TestGormPaymentDB_GetSuccessByOrderID_PendingNotReturned(t *testing.T) {
	g := initTestPaymentDB(t)
	_ = seedOrderForPayment(t, g.DB)
	seedPayment(t, g.DB, func(p *model.Payment) { p.OrderID = 1; p.Status = model.PaymentStatusPending })

	_, err := g.GetSuccessByOrderID(1, 1)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ─── ListByUser ─────────────────────────────────────────────────────────────────

func TestGormPaymentDB_ListByUser(t *testing.T) {
	g := initTestPaymentDB(t)
	seedOrderForPayment(t, g.DB, func(o *model.Order) { o.ID = 10; o.OrderNo = "ORD-010" })
	seedOrderForPayment(t, g.DB, func(o *model.Order) { o.ID = 20; o.OrderNo = "ORD-020" })
	seedOrderForPayment(t, g.DB, func(o *model.Order) { o.ID = 30; o.OrderNo = "ORD-030"; o.UserID = 2 })
	seedPayment(t, g.DB, func(p *model.Payment) { p.OrderID = 10; p.OrderNo = "ORD-010" })
	seedPayment(t, g.DB, func(p *model.Payment) { p.OrderID = 20; p.OrderNo = "ORD-020" })
	seedPayment(t, g.DB, func(p *model.Payment) { p.OrderID = 30; p.OrderNo = "ORD-030"; p.UserID = 2 })

	payments, total, err := g.ListByUser(PaymentListQuery{UserID: 1, Page: 1, PageSize: 10})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, payments, 2)
}

func TestGormPaymentDB_ListByUser_StatusFilter(t *testing.T) {
	g := initTestPaymentDB(t)
	seedOrderForPayment(t, g.DB, func(o *model.Order) { o.ID = 10; o.OrderNo = "ORD-010" })
	seedOrderForPayment(t, g.DB, func(o *model.Order) { o.ID = 20; o.OrderNo = "ORD-020" })
	seedPayment(t, g.DB, func(p *model.Payment) { p.OrderID = 10; p.OrderNo = "ORD-010"; p.Status = model.PaymentStatusSuccess })
	seedPayment(t, g.DB, func(p *model.Payment) { p.OrderID = 20; p.OrderNo = "ORD-020"; p.Status = model.PaymentStatusPending })

	payments, total, err := g.ListByUser(PaymentListQuery{UserID: 1, Status: model.PaymentStatusSuccess, Page: 1, PageSize: 10})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, model.PaymentStatusSuccess, payments[0].Status)
}

func TestGormPaymentDB_ListByUser_ChannelFilter(t *testing.T) {
	g := initTestPaymentDB(t)
	seedOrderForPayment(t, g.DB, func(o *model.Order) { o.ID = 10; o.OrderNo = "ORD-010" })
	seedOrderForPayment(t, g.DB, func(o *model.Order) { o.ID = 20; o.OrderNo = "ORD-020" })
	seedPayment(t, g.DB, func(p *model.Payment) { p.OrderID = 10; p.OrderNo = "ORD-010"; p.Channel = model.PaymentChannelBalance })
	seedPayment(t, g.DB, func(p *model.Payment) { p.OrderID = 20; p.OrderNo = "ORD-020"; p.Channel = model.PaymentChannelMock })

	payments, total, err := g.ListByUser(PaymentListQuery{UserID: 1, Channel: model.PaymentChannelBalance, Page: 1, PageSize: 10})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, model.PaymentChannelBalance, payments[0].Channel)
}

func TestGormPaymentDB_ListByUser_Empty(t *testing.T) {
	g := initTestPaymentDB(t)
	payments, total, err := g.ListByUser(PaymentListQuery{UserID: 999, Page: 1, PageSize: 10})
	assert.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, payments)
}

// ─── Update ─────────────────────────────────────────────────────────────────────

func TestGormPaymentDB_Update(t *testing.T) {
	g := initTestPaymentDB(t)
	seed := seedPayment(t, g.DB)

	seed.Status = model.PaymentStatusSuccess
	now := time.Now()
	seed.PaidAt = &now
	assert.NoError(t, g.Update(seed))

	got, _ := g.GetByPaymentNo(seed.UserID, seed.PaymentNo)
	assert.Equal(t, model.PaymentStatusSuccess, got.Status)
	assert.NotNil(t, got.PaidAt)
}
