package dao

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mall/model"
)

// ─── helpers ────────────────────────────────────────────────────────────────────

func initTestAddressDB(t *testing.T) *GormAddressDB {
	t.Helper()
	db := newTestDB(t, &model.UserAddress{})
	return &GormAddressDB{DB: db}
}

func seedAddress(t *testing.T, db *GormAddressDB, opts ...func(*model.UserAddress)) *model.UserAddress {
	t.Helper()
	a := &model.UserAddress{
		UserID:       1,
		ReceiverName: "Alice",
		Phone:        "13800000001",
		Province:     "Guangdong",
		City:         "Shenzhen",
		District:     "Nanshan",
		Detail:       "1 Main St",
	}
	for _, opt := range opts {
		opt(a)
	}
	require.NoError(t, db.Create(a))
	return a
}

// ─── Create ─────────────────────────────────────────────────────────────────────

func TestGormAddressDB_Create(t *testing.T) {
	g := initTestAddressDB(t)
	a := seedAddress(t, g)
	assert.NotZero(t, a.ID)
}

// ─── GetByID ────────────────────────────────────────────────────────────────────

func TestGormAddressDB_GetByID(t *testing.T) {
	g := initTestAddressDB(t)
	s := seedAddress(t, g)

	got, err := g.GetByID(s.UserID, s.ID)
	assert.NoError(t, err)
	assert.Equal(t, s.ReceiverName, got.ReceiverName)
}

func TestGormAddressDB_GetByID_WrongUser(t *testing.T) {
	g := initTestAddressDB(t)
	s := seedAddress(t, g)

	_, err := g.GetByID(999, s.ID)
	assert.Error(t, err) // record not found
}

// ─── ListByUserID ───────────────────────────────────────────────────────────────

func TestGormAddressDB_ListByUserID(t *testing.T) {
	g := initTestAddressDB(t)
	seedAddress(t, g, func(a *model.UserAddress) { a.IsDefault = false })
	seedAddress(t, g, func(a *model.UserAddress) { a.IsDefault = true })

	list, err := g.ListByUserID(1)
	assert.NoError(t, err)
	assert.Len(t, list, 2)
	// default address should be first (is_default DESC)
	assert.True(t, list[0].IsDefault)
}

func TestGormAddressDB_ListByUserID_Empty(t *testing.T) {
	g := initTestAddressDB(t)
	list, err := g.ListByUserID(999)
	assert.NoError(t, err)
	assert.Empty(t, list)
}

// ─── CountByUserID ──────────────────────────────────────────────────────────────

func TestGormAddressDB_CountByUserID(t *testing.T) {
	g := initTestAddressDB(t)
	seedAddress(t, g, func(a *model.UserAddress) { a.UserID = 1 })
	seedAddress(t, g, func(a *model.UserAddress) { a.UserID = 1 })
	seedAddress(t, g, func(a *model.UserAddress) { a.UserID = 2 })

	count, err := g.CountByUserID(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

// ─── Update ─────────────────────────────────────────────────────────────────────

func TestGormAddressDB_Update(t *testing.T) {
	g := initTestAddressDB(t)
	s := seedAddress(t, g)

	s.ReceiverName = "Bob"
	s.Phone = "13900000002"
	assert.NoError(t, g.Update(s))

	got, _ := g.GetByID(s.UserID, s.ID)
	assert.Equal(t, "Bob", got.ReceiverName)
	assert.Equal(t, "13900000002", got.Phone)
}

func TestGormAddressDB_Update_WrongUser(t *testing.T) {
	// Update checks WHERE id AND user_id — wrong user_id should not update
	g := initTestAddressDB(t)
	s := seedAddress(t, g)

	s.UserID = 999 // wrong user
	s.ReceiverName = "Hacker"
	err := g.Update(s)
	assert.NoError(t, err) // gorm Updates does not error on 0 rows affected

	got, _ := g.GetByID(1, s.ID)
	assert.Equal(t, "Alice", got.ReceiverName) // unchanged
}

// ─── Delete ─────────────────────────────────────────────────────────────────────

func TestGormAddressDB_Delete(t *testing.T) {
	g := initTestAddressDB(t)
	s := seedAddress(t, g)

	rows, err := g.Delete(s.UserID, s.ID)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), rows)
}

func TestGormAddressDB_Delete_WrongUser(t *testing.T) {
	g := initTestAddressDB(t)
	s := seedAddress(t, g)

	rows, err := g.Delete(999, s.ID)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), rows)
}

// ─── ClearDefault ───────────────────────────────────────────────────────────────

func TestGormAddressDB_ClearDefault(t *testing.T) {
	g := initTestAddressDB(t)
	seedAddress(t, g, func(a *model.UserAddress) { a.IsDefault = true })
	seedAddress(t, g, func(a *model.UserAddress) { a.IsDefault = true })

	assert.NoError(t, g.ClearDefault(1))

	list, _ := g.ListByUserID(1)
	for _, a := range list {
		assert.False(t, a.IsDefault)
	}
}

// ─── SetDefault ─────────────────────────────────────────────────────────────────

func TestGormAddressDB_SetDefault(t *testing.T) {
	g := initTestAddressDB(t)
	s := seedAddress(t, g, func(a *model.UserAddress) { a.IsDefault = false })

	rows, err := g.SetDefault(s.UserID, s.ID)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), rows)

	got, _ := g.GetByID(s.UserID, s.ID)
	assert.True(t, got.IsDefault)
}

func TestGormAddressDB_SetDefault_WrongUser(t *testing.T) {
	g := initTestAddressDB(t)
	s := seedAddress(t, g)

	rows, err := g.SetDefault(999, s.ID)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), rows)
}
