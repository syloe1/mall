package dao

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"mall/model"
)

// ─── helpers ────────────────────────────────────────────────────────────────────

func initTestFavoriteDB(t *testing.T) *GormFavoriteDB {
	t.Helper()
	db := newTestDB(t, &model.FavoriteFolder{}, &model.FavoriteItem{}, &model.Product{}, &model.ProductImage{})
	return &GormFavoriteDB{DB: db}
}

func seedFolder(t *testing.T, db *gorm.DB, opts ...func(*model.FavoriteFolder)) *model.FavoriteFolder {
	t.Helper()
	f := &model.FavoriteFolder{
		UserID: 1,
		Name:   "default",
	}
	for _, opt := range opts {
		opt(f)
	}
	require.NoError(t, db.Create(f).Error)
	return f
}

func seedFavoriteItem(t *testing.T, db *gorm.DB, userID, folderID, productID uint) *model.FavoriteItem {
	t.Helper()
	item := &model.FavoriteItem{
		UserID:    userID,
		FolderID:  folderID,
		ProductID: productID,
	}
	require.NoError(t, db.Create(item).Error)
	return item
}

func seedProductForFavorite(t *testing.T, db *gorm.DB) *model.Product {
	t.Helper()
	p := &model.Product{Name: "test", PriceCents: 100, Stock: 10, Status: model.ProductOnSale}
	require.NoError(t, db.Create(p).Error)
	return p
}

// ─── CreateFolder ───────────────────────────────────────────────────────────────

func TestGormFavoriteDB_CreateFolder(t *testing.T) {
	g := initTestFavoriteDB(t)
	f := &model.FavoriteFolder{UserID: 1, Name: "wishlist"}
	assert.NoError(t, g.CreateFolder(f))
	assert.NotZero(t, f.ID)
}

// ─── GetFolderByID ──────────────────────────────────────────────────────────────

func TestGormFavoriteDB_GetFolderByID(t *testing.T) {
	g := initTestFavoriteDB(t)
	f := seedFolder(t, g.DB)

	got, err := g.GetFolderByID(f.UserID, f.ID)
	assert.NoError(t, err)
	assert.Equal(t, "default", got.Name)
}

func TestGormFavoriteDB_GetFolderByID_WrongUser(t *testing.T) {
	g := initTestFavoriteDB(t)
	f := seedFolder(t, g.DB)

	_, err := g.GetFolderByID(999, f.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ─── ListFolders ────────────────────────────────────────────────────────────────

func TestGormFavoriteDB_ListFolders_DefaultFirst(t *testing.T) {
	g := initTestFavoriteDB(t)
	seedFolder(t, g.DB, func(f *model.FavoriteFolder) { f.Name = "regular"; f.IsDefault = false })
	seedFolder(t, g.DB, func(f *model.FavoriteFolder) { f.Name = "default"; f.IsDefault = true })

	list, err := g.ListFolders(1)
	assert.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "default", list[0].Name)
}

func TestGormFavoriteDB_ListFolders_Empty(t *testing.T) {
	g := initTestFavoriteDB(t)
	list, err := g.ListFolders(999)
	assert.NoError(t, err)
	assert.Empty(t, list)
}

// ─── UpdateFolder ───────────────────────────────────────────────────────────────

func TestGormFavoriteDB_UpdateFolder(t *testing.T) {
	g := initTestFavoriteDB(t)
	f := seedFolder(t, g.DB)

	f.Name = "renamed"
	assert.NoError(t, g.UpdateFolder(f))

	got, _ := g.GetFolderByID(f.UserID, f.ID)
	assert.Equal(t, "renamed", got.Name)
}

// ─── DeleteFolder ───────────────────────────────────────────────────────────────

func TestGormFavoriteDB_DeleteFolder(t *testing.T) {
	g := initTestFavoriteDB(t)
	f := seedFolder(t, g.DB)
	seedFavoriteItem(t, g.DB, f.UserID, f.ID, 1)

	assert.NoError(t, g.DeleteFolder(f.UserID, f.ID))

	// folder and its items are gone
	_, err := g.GetFolderByID(f.UserID, f.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestGormFavoriteDB_DeleteFolder_OtherFolderItemsUnaffected(t *testing.T) {
	g := initTestFavoriteDB(t)
	f1 := seedFolder(t, g.DB, func(f *model.FavoriteFolder) { f.Name = "folder1" })
	f2 := seedFolder(t, g.DB, func(f *model.FavoriteFolder) { f.Name = "folder2" })
	seedFavoriteItem(t, g.DB, f1.UserID, f1.ID, 1)
	seedFavoriteItem(t, g.DB, f2.UserID, f2.ID, 2)

	assert.NoError(t, g.DeleteFolder(f1.UserID, f1.ID))

	// f2's item still exists
	got, err := g.IsProductFavorited(f2.UserID, 2)
	assert.NoError(t, err)
	assert.True(t, got)
}

// ─── AddItem ────────────────────────────────────────────────────────────────────

func TestGormFavoriteDB_AddItem(t *testing.T) {
	g := initTestFavoriteDB(t)
	f := seedFolder(t, g.DB)

	assert.NoError(t, g.AddItem(&model.FavoriteItem{UserID: f.UserID, FolderID: f.ID, ProductID: 10}))
}

// ─── DeleteItem ─────────────────────────────────────────────────────────────────

func TestGormFavoriteDB_DeleteItem(t *testing.T) {
	g := initTestFavoriteDB(t)
	f := seedFolder(t, g.DB)
	seedFavoriteItem(t, g.DB, f.UserID, f.ID, 10)

	rows, err := g.DeleteItem(f.UserID, f.ID, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), rows)
}

func TestGormFavoriteDB_DeleteItem_WrongFolder(t *testing.T) {
	g := initTestFavoriteDB(t)
	f := seedFolder(t, g.DB)
	seedFavoriteItem(t, g.DB, f.UserID, f.ID, 10)

	rows, err := g.DeleteItem(f.UserID, 999, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), rows)
}

// ─── GetItem ────────────────────────────────────────────────────────────────────

func TestGormFavoriteDB_GetItem(t *testing.T) {
	g := initTestFavoriteDB(t)
	f := seedFolder(t, g.DB)
	item := seedFavoriteItem(t, g.DB, f.UserID, f.ID, 10)

	got, err := g.GetItem(f.UserID, f.ID, 10)
	assert.NoError(t, err)
	assert.Equal(t, item.ID, got.ID)
}

// ─── ListItems ──────────────────────────────────────────────────────────────────

func TestGormFavoriteDB_ListItems(t *testing.T) {
	g := initTestFavoriteDB(t)
	p := seedProductForFavorite(t, g.DB)
	f := seedFolder(t, g.DB)
	seedFavoriteItem(t, g.DB, f.UserID, f.ID, p.ID)
	seedFavoriteItem(t, g.DB, f.UserID, f.ID, p.ID+1)

	items, total, err := g.ListItems(FavoriteItemQuery{UserID: f.UserID, FolderID: f.ID, Page: 1, PageSize: 10})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, items, 2)
}

func TestGormFavoriteDB_ListItems_Empty(t *testing.T) {
	g := initTestFavoriteDB(t)
	items, total, err := g.ListItems(FavoriteItemQuery{UserID: 1, FolderID: 999, Page: 1, PageSize: 10})
	assert.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, items)
}

// ─── IsProductFavorited ─────────────────────────────────────────────────────────

func TestGormFavoriteDB_IsProductFavorited_True(t *testing.T) {
	g := initTestFavoriteDB(t)
	f := seedFolder(t, g.DB)
	seedFavoriteItem(t, g.DB, f.UserID, f.ID, 42)

	got, err := g.IsProductFavorited(f.UserID, 42)
	assert.NoError(t, err)
	assert.True(t, got)
}

func TestGormFavoriteDB_IsProductFavorited_False(t *testing.T) {
	g := initTestFavoriteDB(t)
	got, err := g.IsProductFavorited(1, 999)
	assert.NoError(t, err)
	assert.False(t, got)
}
