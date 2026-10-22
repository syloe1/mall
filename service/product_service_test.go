package service

import (
	"errors"
	"mall/dao"
	"mall/model"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type mockProductDB struct {
	outbox *mockProductOutboxDB

	createErr                error
	getProductErr            error
	getPublicErr             error
	listPublicErr            error
	listAdminErr             error
	updateErr                error
	addImageErr              error
	deleteImageErr           error
	deleteImagesByProductErr error
	deleteProductErr         error
	searchPublicErr          error
	decreaseStockErr         error
	transactionErr           error

	product         *model.Product
	publicProduct   *model.Product
	listProducts    []model.Product
	listTotal       int64
	deleteImageRows int64
	decreaseRows    int64

	createdProduct *model.Product
	updatedProduct *model.Product
	addedImages    []*model.ProductImage

	lastListPublicQuery dao.ProductListQuery
	lastListAdminQuery  dao.ProductListQuery

	createCalls                int
	updateCalls                int
	addImageCalls              int
	deleteImageCalls           int
	deleteImagesByProductCalls int
	deleteProductCalls         int
	transactionCalls           int
}

func (m *mockProductDB) Create(product *model.Product) error {
	m.createCalls++
	m.createdProduct = product
	if product.ID == 0 {
		product.ID = 100
	}
	return m.createErr
}

func (m *mockProductDB) GetProductById(id uint) (*model.Product, error) {
	if m.getProductErr != nil {
		return nil, m.getProductErr
	}
	if m.product == nil || m.product.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	return m.product, nil
}

func (m *mockProductDB) GetPublicByID(id uint) (*model.Product, error) {
	if m.getPublicErr != nil {
		return nil, m.getPublicErr
	}
	if m.publicProduct == nil || m.publicProduct.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	return m.publicProduct, nil
}

func (m *mockProductDB) ListAdmin(query dao.ProductListQuery) ([]model.Product, int64, error) {
	m.lastListAdminQuery = query
	return m.listProducts, m.listTotal, m.listAdminErr
}

func (m *mockProductDB) ListPublic(query dao.ProductListQuery) ([]model.Product, int64, error) {
	m.lastListPublicQuery = query
	return m.listProducts, m.listTotal, m.listPublicErr
}

func (m *mockProductDB) Update(product *model.Product) error {
	m.updateCalls++
	m.updatedProduct = product
	return m.updateErr
}

func (m *mockProductDB) AddImage(image *model.ProductImage) error {
	m.addImageCalls++
	m.addedImages = append(m.addedImages, image)
	return m.addImageErr
}

func (m *mockProductDB) DeleteImage(productID uint, imageID uint) (int64, error) {
	m.deleteImageCalls++
	return m.deleteImageRows, m.deleteImageErr
}

func (m *mockProductDB) DeleteImagesByProductID(productID uint) error {
	m.deleteImagesByProductCalls++
	return m.deleteImagesByProductErr
}

func (m *mockProductDB) DeleteProductByID(productID uint) error {
	m.deleteProductCalls++
	return m.deleteProductErr
}

func (m *mockProductDB) SearchPublic(query dao.ProductListQuery) ([]model.Product, int64, error) {
	return m.listProducts, m.listTotal, m.searchPublicErr
}

func (m *mockProductDB) DecreaseStock(productID uint, quantity int) (int64, error) {
	return m.decreaseRows, m.decreaseStockErr
}

func (m *mockProductDB) Transaction(fn func(txProductDB dao.ProductDB, txOutboxDB dao.ProductOutboxDB) error) error {
	m.transactionCalls++
	if m.transactionErr != nil {
		return m.transactionErr
	}
	if m.outbox == nil {
		m.outbox = &mockProductOutboxDB{}
	}
	return fn(m, m.outbox)
}

type mockProductOutboxDB struct {
	createErr     error
	events        []*model.ProductOutbox
	createCalls   int
	markPublished int
	markFailed    int
}

func (m *mockProductOutboxDB) Create(event *model.ProductOutbox) error {
	m.createCalls++
	m.events = append(m.events, event)
	return m.createErr
}

func (m *mockProductOutboxDB) ListPending(limit int) ([]model.ProductOutbox, error) {
	return nil, nil
}

func (m *mockProductOutboxDB) MarkPublished(id uint) error {
	m.markPublished++
	return nil
}

func (m *mockProductOutboxDB) MarkFailed(id uint, errMsg string) error {
	m.markFailed++
	return nil
}

type mockProductSearchDB struct {
	searchErr   error
	products    []model.Product
	total       int64
	lastQuery   dao.ProductListQuery
	searchCalls int
}

func (m *mockProductSearchDB) SearchPublic(query dao.ProductListQuery) ([]model.Product, int64, error) {
	m.searchCalls++
	m.lastQuery = query
	return m.products, m.total, m.searchErr
}

func (m *mockProductSearchDB) IndexProduct(product *model.Product) error { return nil }
func (m *mockProductSearchDB) ReplaceAllProducts(products []model.Product) (int, error) {
	return len(products), nil
}
func (m *mockProductSearchDB) DeleteProduct(id uint) error { return nil }

func productFixture(id uint) *model.Product {
	return &model.Product{
		Model:      gorm.Model{ID: id},
		Name:       "apple",
		PriceCents: 1000,
		Stock:      10,
		CategoryID: 1,
		Status:     model.ProductOnSale,
		Images: []model.ProductImage{
			{Model: gorm.Model{ID: 1}, ProductID: id, URL: "old.png", SortOrder: 1},
		},
	}
}

func requireOutbox(t *testing.T, outbox *mockProductOutboxDB, eventType string, productID uint) {
	t.Helper()
	require.Len(t, outbox.events, 1)
	assert.Equal(t, eventType, outbox.events[0].EventType)
	assert.Equal(t, productID, outbox.events[0].ProductID)
	assert.Equal(t, model.ProductOutboxPending, outbox.events[0].Status)
	assert.NotEmpty(t, outbox.events[0].EventID)
}

func TestProductServiceCreate(t *testing.T) {
	t.Run("default_draft_success_with_outbox", func(t *testing.T) {
		outbox := &mockProductOutboxDB{}
		db := &mockProductDB{outbox: outbox}
		svc := NewProductService(db, nil)

		err := svc.Create(CreateProductInput{Name: "draft", PriceCents: 100, Images: []string{"a.png", "b.png"}})

		require.NoError(t, err)
		require.Equal(t, 1, db.createCalls)
		assert.Equal(t, model.ProductDraft, db.createdProduct.Status)
		require.Len(t, db.createdProduct.Images, 2)
		assert.Equal(t, 1, db.createdProduct.Images[0].SortOrder)
		assert.Equal(t, 2, db.createdProduct.Images[1].SortOrder)
		requireOutbox(t, outbox, model.ProductEventCreated, 100)
	})

	t.Run("validation_and_transaction_errors", func(t *testing.T) {
		svc := NewProductService(&mockProductDB{}, nil)
		assert.Error(t, svc.Create(CreateProductInput{Status: "bad"}))
		assert.Error(t, svc.Create(CreateProductInput{Status: model.ProductOnSale}))

		db := &mockProductDB{createErr: errors.New("create failed"), outbox: &mockProductOutboxDB{}}
		svc = NewProductService(db, nil)
		assert.Error(t, svc.Create(CreateProductInput{Name: "x"}))

		db = &mockProductDB{outbox: &mockProductOutboxDB{createErr: errors.New("outbox failed")}}
		svc = NewProductService(db, nil)
		assert.Error(t, svc.Create(CreateProductInput{Name: "x"}))
	})
}

func TestProductServiceGetAndList(t *testing.T) {
	t.Run("get_product_and_public", func(t *testing.T) {
		db := &mockProductDB{product: productFixture(1), publicProduct: productFixture(2)}
		svc := NewProductService(db, nil)

		public, err := svc.GetPublic(2)
		require.NoError(t, err)
		assert.Equal(t, uint(2), public.ID)
		assert.Error(t, func() error { _, err := svc.GetPublic(0); return err }())
		assert.Error(t, func() error { _, err := svc.GetPublic(99); return err }())
	})

	t.Run("list_public_and_admin", func(t *testing.T) {
		db := &mockProductDB{listProducts: []model.Product{*productFixture(1)}, listTotal: 1}
		svc := NewProductService(db, nil)

		public, err := svc.ListPublic(ListPublicProductsInput{Page: -1, PageSize: 200, Keyword: "a"})
		require.NoError(t, err)
		assert.Equal(t, 1, public.Page)
		assert.Equal(t, 100, public.PageSize)
		assert.Equal(t, 1, db.lastListPublicQuery.Page)
		assert.Equal(t, 100, db.lastListPublicQuery.PageSize)

		admin, err := svc.ListAdmin(ListAdminProductsInput{Page: 0, PageSize: 0, Status: model.ProductOnSale})
		require.NoError(t, err)
		assert.Equal(t, 1, admin.Page)
		assert.Equal(t, 20, admin.PageSize)
		assert.Equal(t, model.ProductOnSale, db.lastListAdminQuery.Status)
		assert.Error(t, func() error { _, err := svc.ListAdmin(ListAdminProductsInput{Status: "bad"}); return err }())
	})

	t.Run("list_errors", func(t *testing.T) {
		svc := NewProductService(&mockProductDB{listPublicErr: errors.New("list public failed")}, nil)
		_, err := svc.ListPublic(ListPublicProductsInput{})
		assert.Error(t, err)

		svc = NewProductService(&mockProductDB{listAdminErr: errors.New("list admin failed")}, nil)
		_, err = svc.ListAdmin(ListAdminProductsInput{})
		assert.Error(t, err)
	})
}

func TestProductServiceUpdateAndDelete(t *testing.T) {
	t.Run("update_fields_success_with_outbox", func(t *testing.T) {
		outbox := &mockProductOutboxDB{}
		db := &mockProductDB{product: productFixture(1), outbox: outbox}
		svc := NewProductService(db, nil)

		err := svc.UpdateProduct(UpdateProductInput{ID: 1, Name: "banana", PriceCents: 2000, Status: model.ProductOffSale})

		require.NoError(t, err)
		assert.Equal(t, "banana", db.updatedProduct.Name)
		assert.Equal(t, int64(2000), db.updatedProduct.PriceCents)
		assert.Equal(t, model.ProductOffSale, db.updatedProduct.Status)
		requireOutbox(t, outbox, model.ProductEventUpdated, 1)
	})

	t.Run("replace_images_success", func(t *testing.T) {
		outbox := &mockProductOutboxDB{}
		db := &mockProductDB{product: productFixture(1), outbox: outbox}
		svc := NewProductService(db, nil)

		err := svc.UpdateProduct(UpdateProductInput{ID: 1, Status: model.ProductOnSale, Images: []string{"n1.png", "n2.png"}})

		require.NoError(t, err)
		assert.Equal(t, 1, db.deleteImagesByProductCalls)
		require.Len(t, db.addedImages, 2)
		assert.Equal(t, uint(1), db.addedImages[0].ProductID)
		assert.Equal(t, "n1.png", db.addedImages[0].URL)
		requireOutbox(t, outbox, model.ProductEventUpdated, 1)
	})

	t.Run("update_errors", func(t *testing.T) {
		svc := NewProductService(&mockProductDB{}, nil)
		assert.Error(t, svc.UpdateProduct(UpdateProductInput{}))

		svc = NewProductService(&mockProductDB{getProductErr: gorm.ErrRecordNotFound}, nil)
		assert.Error(t, svc.UpdateProduct(UpdateProductInput{ID: 1}))

		svc = NewProductService(&mockProductDB{product: productFixture(1), updateErr: errors.New("update failed")}, nil)
		assert.Error(t, svc.UpdateProduct(UpdateProductInput{ID: 1}))

		svc = NewProductService(&mockProductDB{product: productFixture(1), deleteImagesByProductErr: errors.New("delete images failed")}, nil)
		assert.Error(t, svc.UpdateProduct(UpdateProductInput{ID: 1, Status: model.ProductOnSale, Images: []string{"x.png"}}))

		svc = NewProductService(&mockProductDB{product: productFixture(1), addImageErr: errors.New("add image failed")}, nil)
		assert.Error(t, svc.UpdateProduct(UpdateProductInput{ID: 1, Status: model.ProductOnSale, Images: []string{"x.png"}}))

		svc = NewProductService(&mockProductDB{product: productFixture(1), outbox: &mockProductOutboxDB{createErr: errors.New("outbox failed")}}, nil)
		assert.Error(t, svc.UpdateProduct(UpdateProductInput{ID: 1}))
	})

	t.Run("delete_success_and_errors", func(t *testing.T) {
		outbox := &mockProductOutboxDB{}
		db := &mockProductDB{product: productFixture(1), outbox: outbox}
		svc := NewProductService(db, nil)
		require.NoError(t, svc.DeleteProduct(1))
		assert.Equal(t, 1, db.deleteProductCalls)
		requireOutbox(t, outbox, model.ProductEventDeleted, 1)

		assert.Error(t, svc.DeleteProduct(0))
		assert.Error(t, NewProductService(&mockProductDB{getProductErr: gorm.ErrRecordNotFound}, nil).DeleteProduct(1))
		assert.Error(t, NewProductService(&mockProductDB{product: productFixture(1), deleteProductErr: errors.New("delete failed")}, nil).DeleteProduct(1))
		assert.Error(t, NewProductService(&mockProductDB{product: productFixture(1), outbox: &mockProductOutboxDB{createErr: errors.New("outbox failed")}}, nil).DeleteProduct(1))
	})
}

func TestProductServiceSearchPublic(t *testing.T) {
	t.Run("nil_search_db", func(t *testing.T) {
		svc := NewProductService(&mockProductDB{}, nil)
		_, err := svc.SearchPublic(SearchPublicProductsInput{})
		assert.Error(t, err)
	})

	t.Run("success_and_query_defaults", func(t *testing.T) {
		search := &mockProductSearchDB{products: []model.Product{*productFixture(1)}, total: 1}
		svc := NewProductService(&mockProductDB{}, search)

		result, err := svc.SearchPublic(SearchPublicProductsInput{Page: -1, PageSize: 200, Keyword: "apple"})

		require.NoError(t, err)
		assert.Equal(t, 1, result.Page)
		assert.Equal(t, 100, result.PageSize)
		assert.Equal(t, int64(1), result.Total)
		assert.Equal(t, 1, search.searchCalls)
		assert.Equal(t, 1, search.lastQuery.Page)
		assert.Equal(t, 100, search.lastQuery.PageSize)
		assert.Equal(t, "apple", search.lastQuery.Keyword)
	})

	t.Run("search_error", func(t *testing.T) {
		search := &mockProductSearchDB{searchErr: errors.New("es down")}
		svc := NewProductService(&mockProductDB{}, search)
		_, err := svc.SearchPublic(SearchPublicProductsInput{})
		assert.Error(t, err)
	})
}
