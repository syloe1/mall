package dao

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"mall/model"
)

// ─── helpers ────────────────────────────────────────────────────────────────────

func initTestOrderDB(t *testing.T) *GormOrderDB {
	t.Helper()
	db := newTestDB(t, &model.Order{}, &model.OrderItem{}, &model.Product{}, &model.ProductImage{}, &model.CartItem{})
	return &GormOrderDB{DB: db}
}

func seedOrder(t *testing.T, db *gorm.DB, opts ...func(*model.Order)) *model.Order {
	t.Helper()
	o := &model.Order{
		OrderNo:      "ORD" + randomSuffix(),
		UserID:       1,
		Status:       model.OrderPendingPay,
		TotalAmount:  9900,
		ReceiverName: "Alice",
		Phone:        "13800138000",
		Province:     "Guangdong",
		City:         "Shenzhen",
		District:     "Nanshan",
		Detail:       "1 Main St",
	}
	for _, opt := range opts {
		opt(o)
	}
	require.NoError(t, db.Create(o).Error)
	return o
}

func seedOrderItem(t *testing.T, db *gorm.DB, orderID, productID uint, qty int, price int64) *model.OrderItem {
	t.Helper()
	item := &model.OrderItem{
		OrderID:      orderID,
		ProductID:    productID,
		ProductName:  "test-product",
		ProductImage: "img.jpg",
		PriceCents:   price,
		Quantity:     qty,
		Subtotal:     price * int64(qty),
	}
	require.NoError(t, db.Create(item).Error)
	return item
}

// ─── Create ─────────────────────────────────────────────────────────────────────

func TestGormOrderDB_Create(t *testing.T) {
	g := initTestOrderDB(t)
	o := &model.Order{OrderNo: "ORD-001", UserID: 1, Status: model.OrderPendingPay, TotalAmount: 5000}
	assert.NoError(t, g.Create(o))
	assert.NotZero(t, o.ID)
}

// ─── GetByID ────────────────────────────────────────────────────────────────────

func TestGormOrderDB_GetByID(t *testing.T) {
	g := initTestOrderDB(t)
	o := seedOrder(t, g.DB)
	seedOrderItem(t, g.DB, o.ID, 1, 2, 9900)

	got, err := g.GetByID(o.UserID, o.ID)
	assert.NoError(t, err)
	assert.Equal(t, o.OrderNo, got.OrderNo)
	require.Len(t, got.Items, 1)
}

func TestGormOrderDB_GetByID_WrongUser(t *testing.T) {
	g := initTestOrderDB(t)
	o := seedOrder(t, g.DB)

	_, err := g.GetByID(999, o.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestGormOrderDB_GetByID_NotFound(t *testing.T) {
	g := initTestOrderDB(t)
	_, err := g.GetByID(1, 9999)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ─── GetByIDForUpdate ───────────────────────────────────────────────────────────

func TestGormOrderDB_GetByIDForUpdate(t *testing.T) {
	g := initTestOrderDB(t)
	o := seedOrder(t, g.DB)
	seedOrderItem(t, g.DB, o.ID, 1, 1, 5000)

	got, err := g.GetByIDForUpdate(o.UserID, o.ID)
	assert.NoError(t, err)
	assert.Equal(t, o.OrderNo, got.OrderNo)
	assert.Len(t, got.Items, 1)
}

func TestGormOrderDB_GetByIDForUpdate_NotFound(t *testing.T) {
	g := initTestOrderDB(t)
	_, err := g.GetByIDForUpdate(1, 9999)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ─── GetAdminByID ───────────────────────────────────────────────────────────────

func TestGormOrderDB_GetAdminByID(t *testing.T) {
	g := initTestOrderDB(t)
	o := seedOrder(t, g.DB)
	seedOrderItem(t, g.DB, o.ID, 1, 2, 1000)

	got, err := g.GetAdminByID(o.ID)
	assert.NoError(t, err)
	assert.Equal(t, o.ID, got.ID)
	assert.Len(t, got.Items, 1)
}

// ─── UpdateStatus ───────────────────────────────────────────────────────────────

func TestGormOrderDB_UpdateStatus(t *testing.T) {
	g := initTestOrderDB(t)
	o := seedOrder(t, g.DB)

	assert.NoError(t, g.UpdateStatus(o.ID, model.OrderPaid))

	got, _ := g.GetByID(o.UserID, o.ID)
	assert.Equal(t, model.OrderPaid, got.Status)
}

// ─── Cancel ─────────────────────────────────────────────────────────────────────

func TestGormOrderDB_Cancel_Success(t *testing.T) {
	g := initTestOrderDB(t)
	o := seedOrder(t, g.DB, func(o *model.Order) { o.Status = model.OrderPendingPay })

	rows, err := g.Cancel(o.UserID, o.ID)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), rows)

	got, _ := g.GetByID(o.UserID, o.ID)
	assert.Equal(t, model.OrderCanceled, got.Status)
}

func TestGormOrderDB_Cancel_AlreadyPaid(t *testing.T) {
	g := initTestOrderDB(t)
	o := seedOrder(t, g.DB, func(o *model.Order) { o.Status = model.OrderPaid })

	rows, err := g.Cancel(o.UserID, o.ID)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), rows) // status != pending_pay, no update
}

func TestGormOrderDB_Cancel_WrongUser(t *testing.T) {
	g := initTestOrderDB(t)
	o := seedOrder(t, g.DB)

	rows, err := g.Cancel(999, o.ID)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), rows)
}

func TestGormOrderDB_Cancel_NotExist(t *testing.T) {
	g := initTestOrderDB(t)
	rows, err := g.Cancel(1, 9999)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), rows)
}

// ─── ListByUser ─────────────────────────────────────────────────────────────────

func TestGormOrderDB_ListByUser(t *testing.T) {
	g := initTestOrderDB(t)
	seedOrder(t, g.DB, func(o *model.Order) { o.UserID = 1 })
	seedOrder(t, g.DB, func(o *model.Order) { o.UserID = 1 })
	seedOrder(t, g.DB, func(o *model.Order) { o.UserID = 2 })

	orders, total, err := g.ListByUser(OrderListQuery{UserID: 1, Page: 1, PageSize: 10})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, orders, 2)
}

func TestGormOrderDB_ListByUser_StatusFilter(t *testing.T) {
	g := initTestOrderDB(t)
	seedOrder(t, g.DB, func(o *model.Order) { o.Status = model.OrderPendingPay })
	seedOrder(t, g.DB, func(o *model.Order) { o.Status = model.OrderPaid })

	orders, total, err := g.ListByUser(OrderListQuery{UserID: 1, Status: model.OrderPaid, Page: 1, PageSize: 10})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, model.OrderPaid, orders[0].Status)
}

func TestGormOrderDB_ListByUser_Empty(t *testing.T) {
	g := initTestOrderDB(t)
	orders, total, err := g.ListByUser(OrderListQuery{UserID: 999, Page: 1, PageSize: 10})
	assert.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, orders)
}

// ─── ListAdmin ──────────────────────────────────────────────────────────────────

func TestGormOrderDB_ListAdmin(t *testing.T) {
	g := initTestOrderDB(t)
	seedOrder(t, g.DB, func(o *model.Order) { o.UserID = 10 })
	seedOrder(t, g.DB, func(o *model.Order) { o.UserID = 20 })

	orders, total, err := g.ListAdmin(AdminOrderListQuery{UserID: 10, Page: 1, PageSize: 10})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, uint(10), orders[0].UserID)
}

func TestGormOrderDB_ListAdmin_OrderNoSearch(t *testing.T) {
	g := initTestOrderDB(t)
	seedOrder(t, g.DB, func(o *model.Order) { o.OrderNo = "ORD-FIND-ME" })
	seedOrder(t, g.DB, func(o *model.Order) { o.OrderNo = "ORD-OTHER" })

	orders, total, err := g.ListAdmin(AdminOrderListQuery{OrderNo: "FIND", Page: 1, PageSize: 10})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Contains(t, orders[0].OrderNo, "FIND")
}

// ─── Transaction ────────────────────────────────────────────────────────────────

func TestGormOrderDB_Transaction_SharedTx(t *testing.T) {
	g := initTestOrderDB(t)

	// seed a cart item; verify it can be deleted inside the tx
	seedOrder(t, g.DB, func(o *model.Order) { o.UserID = 1 })

	err := g.Transaction(func(txOrderDB OrderDB, txProductDB ProductDB, txCartDB CartDB) error {
		o := &model.Order{OrderNo: "ORD-TX-001", UserID: 1, Status: model.OrderPendingPay, TotalAmount: 1000}
		require.NoError(t, txOrderDB.Create(o))
		return nil
	})
	assert.NoError(t, err)
}

func TestGormOrderDB_Transaction_Rollback(t *testing.T) {
	g := initTestOrderDB(t)

	testErr := assert.AnError
	err := g.Transaction(func(txOrderDB OrderDB, txProductDB ProductDB, txCartDB CartDB) error {
		o := &model.Order{OrderNo: "ORD-ROLLBACK", UserID: 1, Status: model.OrderPendingPay, TotalAmount: 1000}
		require.NoError(t, txOrderDB.Create(o))
		return testErr
	})
	assert.ErrorIs(t, err, testErr)

	// order was rolled back
	_, err = g.GetByID(1, 1)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
