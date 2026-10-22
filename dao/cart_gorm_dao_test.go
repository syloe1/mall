package dao

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"mall/model"
)

func initTestCartDB(t *testing.T) *GormCartDB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&model.CartItem{}, &model.Product{}, &model.ProductImage{})
	require.NoError(t, err)

	return &GormCartDB{DB: db}
}

func seedProduct(t *testing.T, db *gorm.DB) *model.Product {
	t.Helper()
	p := &model.Product{
		Name:       "test-product",
		PriceCents: 1000,
		Stock:      10,
		Images: []model.ProductImage{
			{URL: "img1.jpg", SortOrder: 1},
			{URL: "img2.jpg", SortOrder: 2},
		},
	}
	require.NoError(t, db.Create(p).Error)
	return p
}

// ─── CreateItem + GetItem ─────────────────────────────────────────────────────

func TestGormCartDBCreateItem(t *testing.T) {
	g := initTestCartDB(t)
	product := seedProduct(t, g.DB)

	err := g.CreateItem(&model.CartItem{
		UserID:    1,
		ProductID: product.ID,
		Quantity:  2,
		Checked:   true,
	})
	assert.NoError(t, err)

	item, err := g.GetItem(1, product.ID)
	assert.NoError(t, err)
	assert.Equal(t, uint(1), item.UserID)
	assert.Equal(t, product.ID, item.ProductID)
	assert.Equal(t, 2, item.Quantity)
	assert.True(t, item.Checked)
}

func TestGormCartDBGetItem_NotFound(t *testing.T) {
	g := initTestCartDB(t)
	_, err := g.GetItem(999, 999)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ─── UpdateItem ───────────────────────────────────────────────────────────────

func TestGormCartDBUpdateItem(t *testing.T) {
	g := initTestCartDB(t)
	product := seedProduct(t, g.DB)

	require.NoError(t, g.CreateItem(&model.CartItem{
		UserID:    1,
		ProductID: product.ID,
		Quantity:  1,
	}))

	loaded, err := g.GetItem(1, product.ID)
	require.NoError(t, err)

	loaded.Quantity = 5
	loaded.Checked = false
	assert.NoError(t, g.UpdateItem(loaded))

	updated, err := g.GetItem(1, product.ID)
	assert.NoError(t, err)
	assert.Equal(t, 5, updated.Quantity)
	assert.False(t, updated.Checked)
}

// ─── DeleteItem ───────────────────────────────────────────────────────────────

func TestGormCartDBDeleteItem(t *testing.T) {
	g := initTestCartDB(t)
	product := seedProduct(t, g.DB)

	require.NoError(t, g.CreateItem(&model.CartItem{UserID: 1, ProductID: product.ID}))

	rows, err := g.DeleteItem(1, product.ID)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), rows)

	_, err = g.GetItem(1, product.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestGormCartDBDeleteItem_NotExist(t *testing.T) {
	g := initTestCartDB(t)
	rows, err := g.DeleteItem(1, 9999)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), rows)
}

// ─── Clear ────────────────────────────────────────────────────────────────────

func TestGormCartDBClear(t *testing.T) {
	g := initTestCartDB(t)
	product := seedProduct(t, g.DB)

	require.NoError(t, g.CreateItem(&model.CartItem{UserID: 1, ProductID: product.ID}))
	require.NoError(t, g.CreateItem(&model.CartItem{UserID: 1, ProductID: product.ID + 1}))

	assert.NoError(t, g.Clear(1))
	_, err := g.GetItem(1, product.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ─── ListItems ────────────────────────────────────────────────────────────────

func TestGormCartDBListItems(t *testing.T) {
	g := initTestCartDB(t)
	product := seedProduct(t, g.DB)

	require.NoError(t, g.CreateItem(&model.CartItem{UserID: 1, ProductID: product.ID, Quantity: 1}))
	require.NoError(t, g.CreateItem(&model.CartItem{UserID: 1, ProductID: product.ID + 1, Quantity: 2}))

	items, err := g.ListItems(1)
	assert.NoError(t, err)
	assert.Len(t, items, 2)
}

func TestGormCartDBListItems_Empty(t *testing.T) {
	g := initTestCartDB(t)
	items, err := g.ListItems(999)
	assert.NoError(t, err)
	assert.Empty(t, items)
}

// ─── ListCheckedItems ─────────────────────────────────────────────────────────

func TestGormCartDBListCheckedItems(t *testing.T) {
	g := initTestCartDB(t)
	product := seedProduct(t, g.DB)

	require.NoError(t, g.CreateItem(&model.CartItem{UserID: 1, ProductID: product.ID, Checked: true}))
	require.NoError(t, g.CreateItem(&model.CartItem{UserID: 1, ProductID: product.ID + 1, Checked: true}))
	// 用 SetItemChecked 取消勾选第二个（Create 的 bool 零值会被 gorm 跳过）
	_, err := g.SetItemChecked(1, product.ID+1, false)
	require.NoError(t, err)

	items, err := g.ListCheckedItems(1)
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, product.ID, items[0].ProductID)
}

// ─── SetItemChecked ───────────────────────────────────────────────────────────

func TestGormCartDBSetItemChecked(t *testing.T) {
	g := initTestCartDB(t)
	product := seedProduct(t, g.DB)

	require.NoError(t, g.CreateItem(&model.CartItem{UserID: 1, ProductID: product.ID, Checked: true}))

	rows, err := g.SetItemChecked(1, product.ID, false)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), rows)

	item, err := g.GetItem(1, product.ID)
	assert.NoError(t, err)
	assert.False(t, item.Checked)
}

// ─── SetAllChecked ────────────────────────────────────────────────────────────

func TestGormCartDBSetAllChecked(t *testing.T) {
	g := initTestCartDB(t)
	product := seedProduct(t, g.DB)

	require.NoError(t, g.CreateItem(&model.CartItem{UserID: 1, ProductID: product.ID}))
	require.NoError(t, g.CreateItem(&model.CartItem{UserID: 1, ProductID: product.ID + 1}))

	assert.NoError(t, g.SetAllChecked(1, false))

	items, err := g.ListCheckedItems(1)
	assert.NoError(t, err)
	assert.Empty(t, items)
}
