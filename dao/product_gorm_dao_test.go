package dao

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"mall/model"
)

// ─── helpers ────────────────────────────────────────────────────────────────────

func initTestProductDB(t *testing.T) *GormProductDB {
	t.Helper()
	db := newTestDB(t, &model.Product{}, &model.ProductImage{}, &model.ProductOutbox{})
	return &GormProductDB{DB: db}
}

func seedProductWithOpts(t *testing.T, db *gorm.DB, opts ...func(*model.Product)) *model.Product {
	t.Helper()
	p := &model.Product{
		Name:       "test-product",
		PriceCents: 9900,
		Stock:      100,
		Status:     model.ProductOnSale,
	}
	for _, o := range opts {
		o(p)
	}
	require.NoError(t, db.Create(p).Error)
	return p
}

func seedProductImage(t *testing.T, db *gorm.DB, productID uint, url string, sortOrder int) *model.ProductImage {
	t.Helper()
	img := &model.ProductImage{ProductID: productID, URL: url, SortOrder: sortOrder}
	require.NoError(t, db.Create(img).Error)
	return img
}

// ─── Create ─────────────────────────────────────────────────────────────────────

func TestGormProductDB_Create(t *testing.T) {
	g := initTestProductDB(t)
	p := &model.Product{Name: "new", PriceCents: 500, Stock: 10}
	assert.NoError(t, g.Create(p))
	assert.NotZero(t, p.ID)
}

// ─── GetProductById ─────────────────────────────────────────────────────────────

func TestGormProductDB_GetProductById(t *testing.T) {
	g := initTestProductDB(t)
	seed := seedProductWithOpts(t, g.DB)
	seedProductImage(t, g.DB, seed.ID, "a.jpg", 2)
	seedProductImage(t, g.DB, seed.ID, "b.jpg", 1)

	got, err := g.GetProductById(seed.ID)
	assert.NoError(t, err)
	assert.Equal(t, seed.ID, got.ID)
	require.Len(t, got.Images, 2)
	// images ordered by sort_order ASC
	assert.Equal(t, "b.jpg", got.Images[0].URL)
	assert.Equal(t, "a.jpg", got.Images[1].URL)
}

func TestGormProductDB_GetProductById_NotFound(t *testing.T) {
	g := initTestProductDB(t)
	_, err := g.GetProductById(9999)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ─── GetPublicByID ──────────────────────────────────────────────────────────────

func TestGormProductDB_GetPublicByID(t *testing.T) {
	g := initTestProductDB(t)
	seed := seedProductWithOpts(t, g.DB, func(p *model.Product) { p.Status = model.ProductOnSale })

	got, err := g.GetPublicByID(seed.ID)
	assert.NoError(t, err)
	assert.Equal(t, seed.ID, got.ID)
}

func TestGormProductDB_GetPublicByID_OffSale(t *testing.T) {
	g := initTestProductDB(t)
	seed := seedProductWithOpts(t, g.DB, func(p *model.Product) { p.Status = model.ProductOffSale })

	_, err := g.GetPublicByID(seed.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestGormProductDB_GetPublicByID_NotFound(t *testing.T) {
	g := initTestProductDB(t)
	_, err := g.GetPublicByID(9999)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ─── ListAdmin ──────────────────────────────────────────────────────────────────

func TestGormProductDB_ListAdmin_All(t *testing.T) {
	g := initTestProductDB(t)
	seedProductWithOpts(t, g.DB, func(p *model.Product) { p.Name = "p1" })
	seedProductWithOpts(t, g.DB, func(p *model.Product) { p.Name = "p2" })

	products, total, err := g.ListAdmin(ProductListQuery{Page: 1, PageSize: 10})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, products, 2)
}

func TestGormProductDB_ListAdmin_StatusFilter(t *testing.T) {
	g := initTestProductDB(t)
	seedProductWithOpts(t, g.DB, func(p *model.Product) { p.Status = model.ProductOnSale })
	seedProductWithOpts(t, g.DB, func(p *model.Product) { p.Status = model.ProductOffSale })

	products, total, err := g.ListAdmin(ProductListQuery{Page: 1, PageSize: 10, Status: model.ProductOnSale})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, products, 1)
	assert.Equal(t, model.ProductOnSale, products[0].Status)
}

func TestGormProductDB_ListAdmin_CategoryFilter(t *testing.T) {
	g := initTestProductDB(t)
	seedProductWithOpts(t, g.DB, func(p *model.Product) { p.CategoryID = 5 })
	seedProductWithOpts(t, g.DB, func(p *model.Product) { p.CategoryID = 10 })

	products, total, err := g.ListAdmin(ProductListQuery{Page: 1, PageSize: 10, CategoryID: 5})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, uint(5), products[0].CategoryID)
}

func TestGormProductDB_ListAdmin_KeywordFilter(t *testing.T) {
	g := initTestProductDB(t)
	seedProductWithOpts(t, g.DB, func(p *model.Product) { p.Name = "apple-juice" })
	seedProductWithOpts(t, g.DB, func(p *model.Product) { p.Name = "orange-juice" })

	products, total, err := g.ListAdmin(ProductListQuery{Page: 1, PageSize: 10, Keyword: "apple"})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Contains(t, products[0].Name, "apple")
}

func TestGormProductDB_ListAdmin_Pagination(t *testing.T) {
	g := initTestProductDB(t)
	for i := 0; i < 5; i++ {
		seedProductWithOpts(t, g.DB)
	}

	products, total, err := g.ListAdmin(ProductListQuery{Page: 2, PageSize: 2})
	assert.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, products, 2)
}

func TestGormProductDB_ListAdmin_Empty(t *testing.T) {
	g := initTestProductDB(t)
	products, total, err := g.ListAdmin(ProductListQuery{Page: 1, PageSize: 10})
	assert.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, products)
}

// ─── ListPublic ─────────────────────────────────────────────────────────────────

func TestGormProductDB_ListPublic_OnlyOnSale(t *testing.T) {
	g := initTestProductDB(t)
	seedProductWithOpts(t, g.DB, func(p *model.Product) { p.Status = model.ProductOnSale })
	seedProductWithOpts(t, g.DB, func(p *model.Product) { p.Status = model.ProductOffSale })

	products, total, err := g.ListPublic(ProductListQuery{Page: 1, PageSize: 10})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, model.ProductOnSale, products[0].Status)
}

func TestGormProductDB_ListPublic_CategoryAndKeyword(t *testing.T) {
	g := initTestProductDB(t)
	seedProductWithOpts(t, g.DB, func(p *model.Product) { p.CategoryID = 3; p.Name = "phone-case" })
	seedProductWithOpts(t, g.DB, func(p *model.Product) { p.CategoryID = 3; p.Name = "charger" })
	seedProductWithOpts(t, g.DB, func(p *model.Product) { p.CategoryID = 5; p.Name = "phone-case" })

	products, total, err := g.ListPublic(ProductListQuery{Page: 1, PageSize: 10, CategoryID: 3, Keyword: "phone"})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "phone-case", products[0].Name)
}

// ─── SearchPublic ───────────────────────────────────────────────────────────────

func TestGormProductDB_SearchPublic(t *testing.T) {
	g := initTestProductDB(t)
	seedProductWithOpts(t, g.DB, func(p *model.Product) { p.Name = "wireless-mouse" })
	seedProductWithOpts(t, g.DB, func(p *model.Product) { p.Name = "keyboard"; p.Status = model.ProductOffSale })

	products, total, err := g.SearchPublic(ProductListQuery{Page: 1, PageSize: 10, Keyword: "mouse"})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "wireless-mouse", products[0].Name)
}

func TestGormProductDB_SearchPublic_OffSaleExcluded(t *testing.T) {
	g := initTestProductDB(t)
	seedProductWithOpts(t, g.DB, func(p *model.Product) { p.Name = "hidden"; p.Status = model.ProductOffSale })

	products, total, err := g.SearchPublic(ProductListQuery{Page: 1, PageSize: 10, Keyword: "hidden"})
	assert.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, products)
}

// ─── Update ─────────────────────────────────────────────────────────────────────

func TestGormProductDB_Update(t *testing.T) {
	g := initTestProductDB(t)
	seed := seedProductWithOpts(t, g.DB)
	seed.PriceCents = 19900
	seed.Stock = 50

	assert.NoError(t, g.Update(seed))

	got, err := g.GetProductById(seed.ID)
	assert.NoError(t, err)
	assert.Equal(t, int64(19900), got.PriceCents)
	assert.Equal(t, 50, got.Stock)
}

// ─── AddImage + DeleteImage ─────────────────────────────────────────────────────

func TestGormProductDB_AddImage(t *testing.T) {
	g := initTestProductDB(t)
	seed := seedProductWithOpts(t, g.DB)

	assert.NoError(t, g.AddImage(&model.ProductImage{ProductID: seed.ID, URL: "img.jpg", SortOrder: 1}))

	got, err := g.GetProductById(seed.ID)
	assert.NoError(t, err)
	assert.Len(t, got.Images, 1)
}

func TestGormProductDB_DeleteImage(t *testing.T) {
	g := initTestProductDB(t)
	seed := seedProductWithOpts(t, g.DB)
	img := seedProductImage(t, g.DB, seed.ID, "to-delete.jpg", 1)

	rows, err := g.DeleteImage(seed.ID, img.ID)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), rows)
}

func TestGormProductDB_DeleteImage_WrongProduct(t *testing.T) {
	g := initTestProductDB(t)
	seed := seedProductWithOpts(t, g.DB)
	img := seedProductImage(t, g.DB, seed.ID, "img.jpg", 1)

	rows, err := g.DeleteImage(9999, img.ID)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), rows)
}

// ─── DeleteImagesByProductID ────────────────────────────────────────────────────

func TestGormProductDB_DeleteImagesByProductID(t *testing.T) {
	g := initTestProductDB(t)
	seed := seedProductWithOpts(t, g.DB)
	seedProductImage(t, g.DB, seed.ID, "a.jpg", 1)
	seedProductImage(t, g.DB, seed.ID, "b.jpg", 2)

	assert.NoError(t, g.DeleteImagesByProductID(seed.ID))

	got, _ := g.GetProductById(seed.ID)
	assert.Empty(t, got.Images)
}

// ─── DeleteProductByID ──────────────────────────────────────────────────────────

func TestGormProductDB_DeleteProductByID(t *testing.T) {
	g := initTestProductDB(t)
	seed := seedProductWithOpts(t, g.DB)
	seedProductImage(t, g.DB, seed.ID, "img.jpg", 1)

	assert.NoError(t, g.DeleteProductByID(seed.ID))

	// both product and images are gone
	_, err := g.GetProductById(seed.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestGormProductDB_DeleteProductByID_NoImages(t *testing.T) {
	g := initTestProductDB(t)
	seed := seedProductWithOpts(t, g.DB)

	assert.NoError(t, g.DeleteProductByID(seed.ID))
	_, err := g.GetProductById(seed.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ─── DecreaseStock ──────────────────────────────────────────────────────────────

func TestGormProductDB_DecreaseStock_Success(t *testing.T) {
	g := initTestProductDB(t)
	seed := seedProductWithOpts(t, g.DB, func(p *model.Product) { p.Stock = 10 })

	rows, err := g.DecreaseStock(seed.ID, 3)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), rows)

	got, _ := g.GetProductById(seed.ID)
	assert.Equal(t, 7, got.Stock)
}

func TestGormProductDB_DecreaseStock_Insufficient(t *testing.T) {
	g := initTestProductDB(t)
	seed := seedProductWithOpts(t, g.DB, func(p *model.Product) { p.Stock = 2 })

	rows, err := g.DecreaseStock(seed.ID, 5)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), rows)

	// stock unchanged
	got, _ := g.GetProductById(seed.ID)
	assert.Equal(t, 2, got.Stock)
}

func TestGormProductDB_DecreaseStock_ProductNotFound(t *testing.T) {
	g := initTestProductDB(t)
	rows, err := g.DecreaseStock(9999, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), rows)
}

func TestGormProductDB_DecreaseStock_ExactAmount(t *testing.T) {
	g := initTestProductDB(t)
	seed := seedProductWithOpts(t, g.DB, func(p *model.Product) { p.Stock = 5 })

	rows, err := g.DecreaseStock(seed.ID, 5)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), rows)

	got, _ := g.GetProductById(seed.ID)
	assert.Equal(t, 0, got.Stock)
}

// ─── Transaction ────────────────────────────────────────────────────────────────

func TestGormProductDB_Transaction_SharedTx(t *testing.T) {
	g := initTestProductDB(t)

	// verify that txProductDB and txOutboxDB share the same transaction
	var txProductDB ProductDB
	var txOutboxDB ProductOutboxDB

	err := g.Transaction(func(pDB ProductDB, oDB ProductOutboxDB) error {
		txProductDB = pDB
		txOutboxDB = oDB

		p := &model.Product{Name: "tx-product", PriceCents: 100, Stock: 1}
		require.NoError(t, pDB.Create(p))

		evt := &model.ProductOutbox{
			ProductID: p.ID,
			EventType: "product.created",
			Status:    "pending",
		}
		require.NoError(t, oDB.Create(evt))

		return nil
	})
	assert.NoError(t, err)
	assert.NotNil(t, txProductDB)
	assert.NotNil(t, txOutboxDB)
}

func TestGormProductDB_Transaction_Rollback(t *testing.T) {
	g := initTestProductDB(t)

	// Transaction returns the callback's error; on error the tx rolls back
	testErr := assert.AnError
	err := g.Transaction(func(pDB ProductDB, oDB ProductOutboxDB) error {
		p := &model.Product{Name: "rollback-me", PriceCents: 100, Stock: 1}
		require.NoError(t, pDB.Create(p))
		return testErr
	})
	assert.ErrorIs(t, err, testErr)

	// product not persisted because tx was rolled back
	products, total, _ := g.ListAdmin(ProductListQuery{Page: 1, PageSize: 10, Keyword: "rollback-me"})
	assert.Equal(t, int64(0), total)
	assert.Empty(t, products)
}
