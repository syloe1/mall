package service

import (
	"errors"
	"net/http"
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

type mockCartDB struct {
	// base returns
	item         *model.CartItem
	items        []model.CartItem
	rowsAffected int64
	err          error

	// per-method overrides
	getItemFunc        func(userID, productID uint) (*model.CartItem, error)
	createItemFunc     func(item *model.CartItem) error
	updateItemFunc     func(item *model.CartItem) error
	deleteItemFunc     func(userID, productID uint) (int64, error)
	setItemCheckedFunc func(userID, productID uint, checked bool) (int64, error)

	// call recording
	lastCreateItemArg  *model.CartItem
	lastUpdateItemArg  *model.CartItem
	lastDeleteItem     struct{ userID, productID uint }
	lastSetItemChecked struct{ userID, productID uint; checked bool }
	lastSetAllChecked  struct{ userID uint; checked bool }
}

func (m *mockCartDB) GetItem(userID, productID uint) (*model.CartItem, error) {
	if m.getItemFunc != nil {
		return m.getItemFunc(userID, productID)
	}
	return m.item, m.err
}
func (m *mockCartDB) CreateItem(item *model.CartItem) error {
	m.lastCreateItemArg = item
	if m.createItemFunc != nil {
		return m.createItemFunc(item)
	}
	return m.err
}
func (m *mockCartDB) UpdateItem(item *model.CartItem) error {
	m.lastUpdateItemArg = item
	if m.updateItemFunc != nil {
		return m.updateItemFunc(item)
	}
	return m.err
}
func (m *mockCartDB) DeleteItem(userID, productID uint) (int64, error) {
	m.lastDeleteItem.userID = userID
	m.lastDeleteItem.productID = productID
	if m.deleteItemFunc != nil {
		return m.deleteItemFunc(userID, productID)
	}
	return m.rowsAffected, m.err
}
func (m *mockCartDB) Clear(userID uint) error { return m.err }
func (m *mockCartDB) ListItems(userID uint) ([]model.CartItem, error) {
	return m.items, m.err
}
func (m *mockCartDB) ListCheckedItems(userID uint) ([]model.CartItem, error) {
	return m.items, m.err
}
func (m *mockCartDB) SetItemChecked(userID, productID uint, checked bool) (int64, error) {
	m.lastSetItemChecked.userID = userID
	m.lastSetItemChecked.productID = productID
	m.lastSetItemChecked.checked = checked
	if m.setItemCheckedFunc != nil {
		return m.setItemCheckedFunc(userID, productID, checked)
	}
	return m.rowsAffected, m.err
}
func (m *mockCartDB) SetAllChecked(userID uint, checked bool) error {
	m.lastSetAllChecked.userID = userID
	m.lastSetAllChecked.checked = checked
	return m.err
}

type mockProductDBForCart struct {
	product *model.Product
	err     error
}

func (m *mockProductDBForCart) GetProductById(id uint) (*model.Product, error) {
	return m.product, m.err
}

// stubs
func (m *mockProductDBForCart) Create(_ *model.Product) error                { return nil }
func (m *mockProductDBForCart) GetPublicByID(_ uint) (*model.Product, error) { return nil, nil }
func (m *mockProductDBForCart) ListAdmin(_ dao.ProductListQuery) ([]model.Product, int64, error) {
	return nil, 0, nil
}
func (m *mockProductDBForCart) ListPublic(_ dao.ProductListQuery) ([]model.Product, int64, error) {
	return nil, 0, nil
}
func (m *mockProductDBForCart) Update(_ *model.Product) error              { return nil }
func (m *mockProductDBForCart) AddImage(_ *model.ProductImage) error       { return nil }
func (m *mockProductDBForCart) DeleteImage(_, _ uint) (int64, error)       { return 0, nil }
func (m *mockProductDBForCart) DeleteImagesByProductID(_ uint) error       { return nil }
func (m *mockProductDBForCart) DeleteAllImages(_ uint) (int64, error)      { return 0, nil }
func (m *mockProductDBForCart) DeleteProductByID(_ uint) error             { return nil }
func (m *mockProductDBForCart) SearchPublic(_ dao.ProductListQuery) ([]model.Product, int64, error) {
	return nil, 0, nil
}
func (m *mockProductDBForCart) DecreaseStock(_ uint, _ int) (int64, error) { return 1, nil }
func (m *mockProductDBForCart) Transaction(_ func(txProductDB dao.ProductDB, txOutboxDB dao.ProductOutboxDB) error) error {
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// helpers
// ═══════════════════════════════════════════════════════════════════════════════

func newTestCartService() (*CartService, *mockCartDB, *mockProductDBForCart) {
	cartDB := &mockCartDB{}
	prodDB := &mockProductDBForCart{}
	return NewCartService(cartDB, prodDB), cartDB, prodDB
}

func mustBeBizErrorCart(t *testing.T, err error, code int, msgContains string) {
	t.Helper()
	biz, ok := err.(*core.BizError)
	if !ok {
		t.Fatalf("expected *core.BizError, got %T: %v", err, err)
	}
	assert.Equal(t, code, biz.Code, "status code mismatch")
	assert.Contains(t, biz.Msg, msgContains, "message mismatch")
}

func onSaleProductCart(stock int) *model.Product {
	p := &model.Product{Name: "测试商品", PriceCents: 9900, Stock: stock, Status: model.ProductOnSale}
	p.ID = 1
	return p
}

// ═══════════════════════════════════════════════════════════════════════════════
// AddItem
// ═══════════════════════════════════════════════════════════════════════════════

func TestCartService_AddItem(t *testing.T) {

	t.Run("invalid_user_id", func(t *testing.T) {
		svc, _, _ := newTestCartService()
		err := svc.AddItem(AddCartItemInput{UserID: 0, ProductID: 1, Quantity: 1})
		mustBeBizErrorCart(t, err, http.StatusBadRequest, "用户 ID 不合法")
	})

	t.Run("invalid_product_id", func(t *testing.T) {
		svc, _, _ := newTestCartService()
		err := svc.AddItem(AddCartItemInput{UserID: 1, ProductID: 0, Quantity: 1})
		mustBeBizErrorCart(t, err, http.StatusBadRequest, "商品 ID 不合法")
	})

	t.Run("zero_quantity", func(t *testing.T) {
		svc, _, _ := newTestCartService()
		err := svc.AddItem(AddCartItemInput{UserID: 1, ProductID: 1, Quantity: 0})
		mustBeBizErrorCart(t, err, http.StatusBadRequest, "数量必须大于 0")
	})

	t.Run("negative_quantity", func(t *testing.T) {
		svc, _, _ := newTestCartService()
		err := svc.AddItem(AddCartItemInput{UserID: 1, ProductID: 1, Quantity: -1})
		mustBeBizErrorCart(t, err, http.StatusBadRequest, "数量必须大于 0")
	})

	t.Run("product_not_found", func(t *testing.T) {
		svc, _, prodDB := newTestCartService()
		prodDB.err = gorm.ErrRecordNotFound
		err := svc.AddItem(AddCartItemInput{UserID: 1, ProductID: 99, Quantity: 1})
		mustBeBizErrorCart(t, err, http.StatusNotFound, "商品不存在或已下架")
	})

	t.Run("product_db_error", func(t *testing.T) {
		svc, _, prodDB := newTestCartService()
		dbErr := errors.New("db error")
		prodDB.err = dbErr
		err := svc.AddItem(AddCartItemInput{UserID: 1, ProductID: 1, Quantity: 1})
		assert.Equal(t, dbErr, err)
	})

	t.Run("product_not_on_sale", func(t *testing.T) {
		svc, _, prodDB := newTestCartService()
		p := onSaleProductCart(10)
		p.Status = model.ProductOffSale
		prodDB.product = p
		err := svc.AddItem(AddCartItemInput{UserID: 1, ProductID: 1, Quantity: 1})
		mustBeBizErrorCart(t, err, http.StatusBadRequest, "商品已下架")
	})

	t.Run("insufficient_stock", func(t *testing.T) {
		svc, _, prodDB := newTestCartService()
		prodDB.product = onSaleProductCart(3)
		err := svc.AddItem(AddCartItemInput{UserID: 1, ProductID: 1, Quantity: 5})
		mustBeBizErrorCart(t, err, http.StatusBadRequest, "库存不足")
	})

	t.Run("get_item_db_error", func(t *testing.T) {
		svc, cartDB, prodDB := newTestCartService()
		prodDB.product = onSaleProductCart(10)
		dbErr := errors.New("db error")
		cartDB.err = dbErr
		err := svc.AddItem(AddCartItemInput{UserID: 1, ProductID: 1, Quantity: 1})
		assert.Equal(t, dbErr, err)
	})

	t.Run("item_exists_exceeds_stock", func(t *testing.T) {
		svc, cartDB, prodDB := newTestCartService()
		prodDB.product = onSaleProductCart(10)
		cartDB.item = &model.CartItem{UserID: 1, ProductID: 1, Quantity: 8}
		err := svc.AddItem(AddCartItemInput{UserID: 1, ProductID: 1, Quantity: 5})
		mustBeBizErrorCart(t, err, http.StatusBadRequest, "库存不足")
	})

	t.Run("item_exists_update_success", func(t *testing.T) {
		svc, cartDB, prodDB := newTestCartService()
		prodDB.product = onSaleProductCart(10)
		cartDB.item = &model.CartItem{UserID: 1, ProductID: 1, Quantity: 2, Checked: false}
		err := svc.AddItem(AddCartItemInput{UserID: 1, ProductID: 1, Quantity: 3})
		assert.NoError(t, err)
		updated := cartDB.lastUpdateItemArg
		assert.NotNil(t, updated)
		assert.Equal(t, 5, updated.Quantity)
		assert.True(t, updated.Checked)
	})

	t.Run("item_exists_update_db_error", func(t *testing.T) {
		svc, cartDB, prodDB := newTestCartService()
		prodDB.product = onSaleProductCart(10)
		cartDB.item = &model.CartItem{Quantity: 2}
		updateErr := errors.New("update failed")
		cartDB.updateItemFunc = func(item *model.CartItem) error { return updateErr }
		err := svc.AddItem(AddCartItemInput{UserID: 1, ProductID: 1, Quantity: 3})
		assert.Equal(t, updateErr, err)
	})

	t.Run("item_not_found_create_success", func(t *testing.T) {
		svc, cartDB, prodDB := newTestCartService()
		prodDB.product = onSaleProductCart(10)
		cartDB.item = nil
		// GetItem: not found, CreateItem: succeed (override because m.err is shared)
		cartDB.err = gorm.ErrRecordNotFound
		cartDB.createItemFunc = func(item *model.CartItem) error { return nil }
		err := svc.AddItem(AddCartItemInput{UserID: 1, ProductID: 1, Quantity: 3})
		assert.NoError(t, err)
		created := cartDB.lastCreateItemArg
		assert.NotNil(t, created)
		assert.Equal(t, uint(1), created.UserID)
		assert.Equal(t, uint(1), created.ProductID)
		assert.Equal(t, 3, created.Quantity)
		assert.True(t, created.Checked)
	})

	t.Run("item_not_found_create_db_error", func(t *testing.T) {
		svc, cartDB, prodDB := newTestCartService()
		prodDB.product = onSaleProductCart(10)
		cartDB.item = nil
		cartDB.err = gorm.ErrRecordNotFound
		createErr := errors.New("create failed")
		cartDB.createItemFunc = func(item *model.CartItem) error { return createErr }
		err := svc.AddItem(AddCartItemInput{UserID: 1, ProductID: 1, Quantity: 3})
		assert.Equal(t, createErr, err)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// ListItems
// ═══════════════════════════════════════════════════════════════════════════════

func TestCartService_ListItems(t *testing.T) {

	t.Run("invalid_user_id", func(t *testing.T) {
		svc, _, _ := newTestCartService()
		items, err := svc.ListItems(0)
		assert.Nil(t, items)
		mustBeBizErrorCart(t, err, http.StatusBadRequest, "用户 ID 不合法")
	})

	t.Run("success", func(t *testing.T) {
		svc, cartDB, _ := newTestCartService()
		want := []model.CartItem{{Quantity: 1}, {Quantity: 2}}
		cartDB.items = want
		got, err := svc.ListItems(1)
		assert.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("db_error", func(t *testing.T) {
		svc, cartDB, _ := newTestCartService()
		dbErr := errors.New("db down")
		cartDB.err = dbErr
		items, err := svc.ListItems(1)
		assert.Nil(t, items)
		assert.Equal(t, dbErr, err)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// UpdateItem
// ═══════════════════════════════════════════════════════════════════════════════

func TestCartService_UpdateItem(t *testing.T) {

	t.Run("invalid_user_id", func(t *testing.T) {
		svc, _, _ := newTestCartService()
		err := svc.UpdateItem(UpdateCartItemInput{UserID: 0, ProductID: 1, Quantity: 1})
		mustBeBizErrorCart(t, err, http.StatusBadRequest, "请求参数不合法")
	})

	t.Run("invalid_product_id", func(t *testing.T) {
		svc, _, _ := newTestCartService()
		err := svc.UpdateItem(UpdateCartItemInput{UserID: 1, ProductID: 0, Quantity: 1})
		mustBeBizErrorCart(t, err, http.StatusBadRequest, "请求参数不合法")
	})

	t.Run("invalid_quantity", func(t *testing.T) {
		svc, _, _ := newTestCartService()
		err := svc.UpdateItem(UpdateCartItemInput{UserID: 1, ProductID: 1, Quantity: 0})
		mustBeBizErrorCart(t, err, http.StatusBadRequest, "请求参数不合法")
	})

	t.Run("item_not_found", func(t *testing.T) {
		svc, cartDB, _ := newTestCartService()
		cartDB.err = gorm.ErrRecordNotFound
		err := svc.UpdateItem(UpdateCartItemInput{UserID: 1, ProductID: 99, Quantity: 1})
		mustBeBizErrorCart(t, err, http.StatusNotFound, "购物车商品不存在")
	})

	t.Run("get_item_db_error", func(t *testing.T) {
		svc, cartDB, _ := newTestCartService()
		dbErr := errors.New("db error")
		cartDB.err = dbErr
		err := svc.UpdateItem(UpdateCartItemInput{UserID: 1, ProductID: 1, Quantity: 1})
		assert.Equal(t, dbErr, err)
	})

	t.Run("get_product_db_error", func(t *testing.T) {
		svc, cartDB, prodDB := newTestCartService()
		cartDB.item = &model.CartItem{Quantity: 1}
		dbErr := errors.New("product db down")
		prodDB.err = dbErr
		err := svc.UpdateItem(UpdateCartItemInput{UserID: 1, ProductID: 1, Quantity: 2})
		assert.Equal(t, dbErr, err)
	})

	t.Run("quantity_exceeds_stock", func(t *testing.T) {
		svc, cartDB, prodDB := newTestCartService()
		cartDB.item = &model.CartItem{Quantity: 1}
		prodDB.product = onSaleProductCart(3)
		err := svc.UpdateItem(UpdateCartItemInput{UserID: 1, ProductID: 1, Quantity: 5})
		mustBeBizErrorCart(t, err, http.StatusBadRequest, "库存不足")
	})

	t.Run("success", func(t *testing.T) {
		svc, cartDB, prodDB := newTestCartService()
		cartDB.item = &model.CartItem{UserID: 1, ProductID: 1, Quantity: 1}
		prodDB.product = onSaleProductCart(10)
		err := svc.UpdateItem(UpdateCartItemInput{UserID: 1, ProductID: 1, Quantity: 5})
		assert.NoError(t, err)
		updated := cartDB.lastUpdateItemArg
		assert.NotNil(t, updated)
		assert.Equal(t, 5, updated.Quantity)
	})

	t.Run("update_item_db_error", func(t *testing.T) {
		svc, cartDB, prodDB := newTestCartService()
		cartDB.item = &model.CartItem{Quantity: 1}
		prodDB.product = onSaleProductCart(10)
		updateErr := errors.New("update failed")
		cartDB.updateItemFunc = func(item *model.CartItem) error { return updateErr }
		err := svc.UpdateItem(UpdateCartItemInput{UserID: 1, ProductID: 1, Quantity: 5})
		assert.Equal(t, updateErr, err)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// DeleteItem
// ═══════════════════════════════════════════════════════════════════════════════

func TestCartService_DeleteItem(t *testing.T) {

	t.Run("success", func(t *testing.T) {
		svc, cartDB, _ := newTestCartService()
		cartDB.rowsAffected = 1
		err := svc.DeleteItem(DeleteCartItemInput{UserID: 1, ProductID: 1})
		assert.NoError(t, err)
		assert.Equal(t, uint(1), cartDB.lastDeleteItem.userID)
		assert.Equal(t, uint(1), cartDB.lastDeleteItem.productID)
	})

	t.Run("not_found", func(t *testing.T) {
		svc, cartDB, _ := newTestCartService()
		cartDB.rowsAffected = 0
		err := svc.DeleteItem(DeleteCartItemInput{UserID: 1, ProductID: 99})
		mustBeBizErrorCart(t, err, http.StatusNotFound, "购物车商品不存在")
	})

	t.Run("db_error", func(t *testing.T) {
		svc, cartDB, _ := newTestCartService()
		dbErr := errors.New("delete failed")
		cartDB.err = dbErr
		err := svc.DeleteItem(DeleteCartItemInput{UserID: 1, ProductID: 1})
		assert.Equal(t, dbErr, err)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// Clear
// ═══════════════════════════════════════════════════════════════════════════════

func TestCartService_Clear(t *testing.T) {

	t.Run("invalid_user_id", func(t *testing.T) {
		svc, _, _ := newTestCartService()
		err := svc.Clear(0)
		mustBeBizErrorCart(t, err, http.StatusBadRequest, "用户 ID 不合法")
	})

	t.Run("success", func(t *testing.T) {
		svc, _, _ := newTestCartService()
		err := svc.Clear(1)
		assert.NoError(t, err)
	})

	t.Run("db_error", func(t *testing.T) {
		svc, cartDB, _ := newTestCartService()
		dbErr := errors.New("clear failed")
		cartDB.err = dbErr
		err := svc.Clear(1)
		assert.Equal(t, dbErr, err)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// SetItemChecked
// ═══════════════════════════════════════════════════════════════════════════════

func TestCartService_SetItemChecked(t *testing.T) {

	t.Run("item_not_found", func(t *testing.T) {
		svc, cartDB, _ := newTestCartService()
		cartDB.err = gorm.ErrRecordNotFound
		err := svc.SetItemChecked(SetCartItemCheckedInput{UserID: 1, ProductID: 99, Checked: true})
		mustBeBizErrorCart(t, err, http.StatusNotFound, "购物车商品不存在")
	})

	t.Run("get_item_db_error", func(t *testing.T) {
		svc, cartDB, _ := newTestCartService()
		dbErr := errors.New("db error")
		cartDB.err = dbErr
		err := svc.SetItemChecked(SetCartItemCheckedInput{UserID: 1, ProductID: 1, Checked: true})
		assert.Equal(t, dbErr, err)
	})

	t.Run("check_product_not_on_sale", func(t *testing.T) {
		svc, cartDB, prodDB := newTestCartService()
		cartDB.item = &model.CartItem{UserID: 1, ProductID: 1, Quantity: 2}
		p := onSaleProductCart(10)
		p.Status = model.ProductOffSale
		prodDB.product = p
		err := svc.SetItemChecked(SetCartItemCheckedInput{UserID: 1, ProductID: 1, Checked: true})
		mustBeBizErrorCart(t, err, http.StatusBadRequest, "该商品不可勾选")
	})

	t.Run("check_insufficient_stock", func(t *testing.T) {
		svc, cartDB, prodDB := newTestCartService()
		cartDB.item = &model.CartItem{UserID: 1, ProductID: 1, Quantity: 5}
		prodDB.product = onSaleProductCart(3)
		err := svc.SetItemChecked(SetCartItemCheckedInput{UserID: 1, ProductID: 1, Checked: true})
		mustBeBizErrorCart(t, err, http.StatusBadRequest, "该商品不可勾选")
	})

	t.Run("check_product_db_error", func(t *testing.T) {
		svc, cartDB, prodDB := newTestCartService()
		cartDB.item = &model.CartItem{Quantity: 1}
		dbErr := errors.New("product db down")
		prodDB.err = dbErr
		err := svc.SetItemChecked(SetCartItemCheckedInput{UserID: 1, ProductID: 1, Checked: true})
		assert.Equal(t, dbErr, err)
	})

	t.Run("uncheck_skips_product_check", func(t *testing.T) {
		svc, cartDB, _ := newTestCartService()
		cartDB.item = &model.CartItem{Quantity: 1}
		cartDB.rowsAffected = 1
		err := svc.SetItemChecked(SetCartItemCheckedInput{UserID: 1, ProductID: 1, Checked: false})
		assert.NoError(t, err)
		assert.False(t, cartDB.lastSetItemChecked.checked)
	})

	t.Run("set_checked_db_error", func(t *testing.T) {
		svc, cartDB, _ := newTestCartService()
		cartDB.item = &model.CartItem{Quantity: 1}
		dbErr := errors.New("update failed")
		cartDB.err = dbErr
		err := svc.SetItemChecked(SetCartItemCheckedInput{UserID: 1, ProductID: 1, Checked: false})
		assert.Equal(t, dbErr, err)
	})

	t.Run("success_checked", func(t *testing.T) {
		svc, cartDB, prodDB := newTestCartService()
		cartDB.item = &model.CartItem{UserID: 1, ProductID: 1, Quantity: 2}
		prodDB.product = onSaleProductCart(10)
		cartDB.rowsAffected = 1
		err := svc.SetItemChecked(SetCartItemCheckedInput{UserID: 1, ProductID: 1, Checked: true})
		assert.NoError(t, err)
		assert.True(t, cartDB.lastSetItemChecked.checked)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// SetAllChecked
// ═══════════════════════════════════════════════════════════════════════════════

func TestCartService_SetAllChecked(t *testing.T) {

	t.Run("invalid_user_id", func(t *testing.T) {
		svc, _, _ := newTestCartService()
		err := svc.SetAllChecked(SetCartAllCheckedInput{UserID: 0, Checked: true})
		mustBeBizErrorCart(t, err, http.StatusBadRequest, "用户 ID 不合法")
	})

	t.Run("success", func(t *testing.T) {
		svc, cartDB, _ := newTestCartService()
		err := svc.SetAllChecked(SetCartAllCheckedInput{UserID: 1, Checked: true})
		assert.NoError(t, err)
		assert.Equal(t, uint(1), cartDB.lastSetAllChecked.userID)
		assert.True(t, cartDB.lastSetAllChecked.checked)
	})

	t.Run("db_error", func(t *testing.T) {
		svc, cartDB, _ := newTestCartService()
		dbErr := errors.New("set all checked failed")
		cartDB.err = dbErr
		err := svc.SetAllChecked(SetCartAllCheckedInput{UserID: 1, Checked: false})
		assert.Equal(t, dbErr, err)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// ListCheckedItems
// ═══════════════════════════════════════════════════════════════════════════════

func TestCartService_ListCheckedItems(t *testing.T) {

	t.Run("invalid_user_id", func(t *testing.T) {
		svc, _, _ := newTestCartService()
		items, err := svc.ListCheckedItems(0)
		assert.Nil(t, items)
		mustBeBizErrorCart(t, err, http.StatusBadRequest, "用户 ID 不合法")
	})

	t.Run("success", func(t *testing.T) {
		svc, cartDB, _ := newTestCartService()
		want := []model.CartItem{{Quantity: 1, Checked: true}}
		cartDB.items = want
		got, err := svc.ListCheckedItems(1)
		assert.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("db_error", func(t *testing.T) {
		svc, cartDB, _ := newTestCartService()
		dbErr := errors.New("db down")
		cartDB.err = dbErr
		items, err := svc.ListCheckedItems(1)
		assert.Nil(t, items)
		assert.Equal(t, dbErr, err)
	})
}
