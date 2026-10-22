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
// local mocks — per-method func overrides for precise sequential control
// ═══════════════════════════════════════════════════════════════════════════════

type mockFavoriteDB struct {
	// base returns (used when the corresponding func is nil)
	folder       *model.FavoriteFolder
	folders      []*model.FavoriteFolder
	item         *model.FavoriteItem
	items        []*model.FavoriteItem
	total        int64
	rowsAffected int64
	isFavorited  bool
	err          error

	// per-method overrides — if non-nil the method delegates to this func
	getFolderByIDFunc func(userID, folderID uint) (*model.FavoriteFolder, error)
	updateFolderFunc  func(folder *model.FavoriteFolder) error
	deleteFolderFunc  func(userID, folderID uint) error
	addItemFunc       func(item *model.FavoriteItem) error
	getItemFunc       func(userID, folderID, productID uint) (*model.FavoriteItem, error)

	// call recording
	lastCreateFolderArg *model.FavoriteFolder
	lastUpdateFolderArg *model.FavoriteFolder
	lastDeleteFolder    struct{ userID, folderID uint }
	lastAddItemArg      *model.FavoriteItem
	lastDeleteItem      struct{ userID, folderID, productID uint }
	lastListItemsQuery  dao.FavoriteItemQuery
	lastIsFavorited     struct{ userID, productID uint }
}

func (m *mockFavoriteDB) CreateFolder(folder *model.FavoriteFolder) error {
	m.lastCreateFolderArg = folder
	return m.err
}

func (m *mockFavoriteDB) GetFolderByID(userID, folderID uint) (*model.FavoriteFolder, error) {
	if m.getFolderByIDFunc != nil {
		return m.getFolderByIDFunc(userID, folderID)
	}
	return m.folder, m.err
}

func (m *mockFavoriteDB) ListFolders(userID uint) ([]*model.FavoriteFolder, error) {
	return m.folders, m.err
}

func (m *mockFavoriteDB) UpdateFolder(folder *model.FavoriteFolder) error {
	m.lastUpdateFolderArg = folder
	if m.updateFolderFunc != nil {
		return m.updateFolderFunc(folder)
	}
	return m.err
}

func (m *mockFavoriteDB) DeleteFolder(userID, folderID uint) error {
	m.lastDeleteFolder.userID = userID
	m.lastDeleteFolder.folderID = folderID
	if m.deleteFolderFunc != nil {
		return m.deleteFolderFunc(userID, folderID)
	}
	return m.err
}

func (m *mockFavoriteDB) AddItem(item *model.FavoriteItem) error {
	m.lastAddItemArg = item
	if m.addItemFunc != nil {
		return m.addItemFunc(item)
	}
	return m.err
}

func (m *mockFavoriteDB) DeleteItem(userID, folderID, productID uint) (int64, error) {
	m.lastDeleteItem.userID = userID
	m.lastDeleteItem.folderID = folderID
	m.lastDeleteItem.productID = productID
	return m.rowsAffected, m.err
}

func (m *mockFavoriteDB) GetItem(userID, folderID, productID uint) (*model.FavoriteItem, error) {
	if m.getItemFunc != nil {
		return m.getItemFunc(userID, folderID, productID)
	}
	return m.item, m.err
}

func (m *mockFavoriteDB) ListItems(query dao.FavoriteItemQuery) ([]*model.FavoriteItem, int64, error) {
	m.lastListItemsQuery = query
	return m.items, m.total, m.err
}

func (m *mockFavoriteDB) IsProductFavorited(userID, productID uint) (bool, error) {
	m.lastIsFavorited.userID = userID
	m.lastIsFavorited.productID = productID
	return m.isFavorited, m.err
}

type mockProductDBForFavorite struct {
	product *model.Product
	err     error
}

func (m *mockProductDBForFavorite) GetProductById(id uint) (*model.Product, error) {
	return m.product, m.err
}

// --- unused ProductDB methods (stubs) ---
func (m *mockProductDBForFavorite) Create(_ *model.Product) error                   { return nil }
func (m *mockProductDBForFavorite) GetPublicByID(_ uint) (*model.Product, error)    { return nil, nil }
func (m *mockProductDBForFavorite) ListAdmin(_ dao.ProductListQuery) ([]model.Product, int64, error) {
	return nil, 0, nil
}
func (m *mockProductDBForFavorite) ListPublic(_ dao.ProductListQuery) ([]model.Product, int64, error) {
	return nil, 0, nil
}
func (m *mockProductDBForFavorite) Update(_ *model.Product) error                    { return nil }
func (m *mockProductDBForFavorite) AddImage(_ *model.ProductImage) error             { return nil }
func (m *mockProductDBForFavorite) DeleteImage(_, _ uint) (int64, error)             { return 0, nil }
func (m *mockProductDBForFavorite) DeleteImagesByProductID(_ uint) error             { return nil }
func (m *mockProductDBForFavorite) DeleteAllImages(_ uint) (int64, error)            { return 0, nil }
func (m *mockProductDBForFavorite) DeleteProductByID(_ uint) error                   { return nil }
func (m *mockProductDBForFavorite) SearchPublic(_ dao.ProductListQuery) ([]model.Product, int64, error) {
	return nil, 0, nil
}
func (m *mockProductDBForFavorite) DecreaseStock(_ uint, _ int) (int64, error)       { return 0, nil }
func (m *mockProductDBForFavorite) Transaction(_ func(txProductDB dao.ProductDB, txOutboxDB dao.ProductOutboxDB) error) error {
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// helpers
// ═══════════════════════════════════════════════════════════════════════════════

func newTestFavoriteService() (*FavoriteService, *mockFavoriteDB, *mockProductDBForFavorite) {
	favDB := &mockFavoriteDB{}
	prodDB := &mockProductDBForFavorite{}
	return NewFavoriteService(favDB, prodDB), favDB, prodDB
}

func mustBeBizError(t *testing.T, err error, code int, msgContains string) {
	t.Helper()
	biz, ok := err.(*core.BizError)
	if !ok {
		t.Fatalf("expected *core.BizError, got %T: %v", err, err)
	}
	assert.Equal(t, code, biz.Code, "status code mismatch")
	assert.Contains(t, biz.Msg, msgContains, "message mismatch")
}

// ═══════════════════════════════════════════════════════════════════════════════
// CreateFolder
// ═══════════════════════════════════════════════════════════════════════════════

func TestFavoriteService_CreateFolder(t *testing.T) {

	t.Run("invalid_user_id", func(t *testing.T) {
		svc, _, _ := newTestFavoriteService()
		err := svc.CreateFolder(CreateFavoriteFolderInput{UserID: 0, Name: "work"})
		mustBeBizError(t, err, http.StatusBadRequest, "用户 ID 不合法")
	})

	t.Run("empty_name", func(t *testing.T) {
		svc, _, _ := newTestFavoriteService()
		err := svc.CreateFolder(CreateFavoriteFolderInput{UserID: 1, Name: ""})
		mustBeBizError(t, err, http.StatusBadRequest, "请填写收藏夹名称")
	})

	t.Run("whitespace_name", func(t *testing.T) {
		svc, _, _ := newTestFavoriteService()
		err := svc.CreateFolder(CreateFavoriteFolderInput{UserID: 1, Name: "   "})
		mustBeBizError(t, err, http.StatusBadRequest, "请填写收藏夹名称")
	})

	t.Run("success", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		err := svc.CreateFolder(CreateFavoriteFolderInput{UserID: 1, Name: "  Work  "})
		assert.NoError(t, err)

		f := favDB.lastCreateFolderArg
		assert.NotNil(t, f)
		assert.Equal(t, uint(1), f.UserID)
		assert.Equal(t, "Work", f.Name)
		assert.False(t, f.IsDefault)
	})

	t.Run("db_error", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		dbErr := errors.New("insert failed")
		favDB.err = dbErr
		err := svc.CreateFolder(CreateFavoriteFolderInput{UserID: 1, Name: "Work"})
		assert.Equal(t, dbErr, err)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// ListFolders
// ═══════════════════════════════════════════════════════════════════════════════

func TestFavoriteService_ListFolders(t *testing.T) {

	t.Run("invalid_user_id", func(t *testing.T) {
		svc, _, _ := newTestFavoriteService()
		folders, err := svc.ListFolders(0)
		assert.Nil(t, folders)
		mustBeBizError(t, err, http.StatusBadRequest, "用户 ID 不合法")
	})

	t.Run("success", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		want := []*model.FavoriteFolder{{Name: "A"}, {Name: "B"}}
		favDB.folders = want

		got, err := svc.ListFolders(1)
		assert.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("db_error", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		dbErr := errors.New("db down")
		favDB.err = dbErr
		folders, err := svc.ListFolders(1)
		assert.Nil(t, folders)
		assert.Equal(t, dbErr, err)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// UpdateFolder
// ═══════════════════════════════════════════════════════════════════════════════

func TestFavoriteService_UpdateFolder(t *testing.T) {

	t.Run("invalid_user_id", func(t *testing.T) {
		svc, _, _ := newTestFavoriteService()
		err := svc.UpdateFolder(UpdateFavoriteFolderInput{UserID: 0, FolderID: 1, Name: "x"})
		mustBeBizError(t, err, http.StatusBadRequest, "用户 ID 不合法")
	})

	t.Run("invalid_folder_id", func(t *testing.T) {
		svc, _, _ := newTestFavoriteService()
		err := svc.UpdateFolder(UpdateFavoriteFolderInput{UserID: 1, FolderID: 0, Name: "x"})
		mustBeBizError(t, err, http.StatusBadRequest, "收藏夹 ID 不合法")
	})

	t.Run("empty_name", func(t *testing.T) {
		svc, _, _ := newTestFavoriteService()
		err := svc.UpdateFolder(UpdateFavoriteFolderInput{UserID: 1, FolderID: 1, Name: ""})
		mustBeBizError(t, err, http.StatusBadRequest, "请填写收藏夹名称")
	})

	t.Run("folder_not_found", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		favDB.err = gorm.ErrRecordNotFound
		err := svc.UpdateFolder(UpdateFavoriteFolderInput{UserID: 1, FolderID: 99, Name: "x"})
		mustBeBizError(t, err, http.StatusNotFound, "收藏夹不存在")
	})

	t.Run("get_folder_db_error", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		dbErr := errors.New("db timeout")
		favDB.err = dbErr
		err := svc.UpdateFolder(UpdateFavoriteFolderInput{UserID: 1, FolderID: 1, Name: "x"})
		assert.Equal(t, dbErr, err)
	})

	t.Run("success", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		favDB.folder = &model.FavoriteFolder{Name: "old"}
		err := svc.UpdateFolder(UpdateFavoriteFolderInput{UserID: 1, FolderID: 2, Name: "  New  "})
		assert.NoError(t, err)

		updated := favDB.lastUpdateFolderArg
		assert.NotNil(t, updated)
		assert.Equal(t, "New", updated.Name)
	})

	t.Run("update_db_error", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		// GetFolderByID succeeds, UpdateFolder fails
		favDB.folder = &model.FavoriteFolder{Name: "old"}
		updateErr := errors.New("update failed")
		favDB.updateFolderFunc = func(folder *model.FavoriteFolder) error {
			return updateErr
		}

		err := svc.UpdateFolder(UpdateFavoriteFolderInput{UserID: 1, FolderID: 2, Name: "  New  "})
		assert.Equal(t, updateErr, err)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// DeleteFolder
// ═══════════════════════════════════════════════════════════════════════════════

func TestFavoriteService_DeleteFolder(t *testing.T) {

	t.Run("invalid_user_id", func(t *testing.T) {
		svc, _, _ := newTestFavoriteService()
		err := svc.DeleteFolder(DeleteFavoriteFolderInput{UserID: 0, FolderID: 1})
		mustBeBizError(t, err, http.StatusBadRequest, "用户 ID 不合法")
	})

	t.Run("invalid_folder_id", func(t *testing.T) {
		svc, _, _ := newTestFavoriteService()
		err := svc.DeleteFolder(DeleteFavoriteFolderInput{UserID: 1, FolderID: 0})
		mustBeBizError(t, err, http.StatusBadRequest, "收藏夹 ID 不合法")
	})

	t.Run("folder_not_found", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		favDB.err = gorm.ErrRecordNotFound
		err := svc.DeleteFolder(DeleteFavoriteFolderInput{UserID: 1, FolderID: 99})
		mustBeBizError(t, err, http.StatusNotFound, "收藏夹不存在")
	})

	t.Run("get_folder_db_error", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		dbErr := errors.New("db error")
		favDB.err = dbErr
		err := svc.DeleteFolder(DeleteFavoriteFolderInput{UserID: 1, FolderID: 1})
		assert.Equal(t, dbErr, err)
	})

	t.Run("cannot_delete_default", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		favDB.folder = &model.FavoriteFolder{IsDefault: true}
		err := svc.DeleteFolder(DeleteFavoriteFolderInput{UserID: 1, FolderID: 1})
		mustBeBizError(t, err, http.StatusBadRequest, "默认收藏夹不能删除")
	})

	t.Run("success", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		favDB.folder = &model.FavoriteFolder{IsDefault: false}
		err := svc.DeleteFolder(DeleteFavoriteFolderInput{UserID: 1, FolderID: 2})
		assert.NoError(t, err)
		assert.Equal(t, uint(1), favDB.lastDeleteFolder.userID)
		assert.Equal(t, uint(2), favDB.lastDeleteFolder.folderID)
	})

	t.Run("delete_db_error", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		favDB.folder = &model.FavoriteFolder{IsDefault: false}
		deleteErr := errors.New("delete failed")
		favDB.deleteFolderFunc = func(userID, folderID uint) error {
			return deleteErr
		}

		err := svc.DeleteFolder(DeleteFavoriteFolderInput{UserID: 1, FolderID: 2})
		assert.Equal(t, deleteErr, err)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// AddItem
// ═══════════════════════════════════════════════════════════════════════════════

func TestFavoriteService_AddItem(t *testing.T) {

	t.Run("invalid_user_id", func(t *testing.T) {
		svc, _, _ := newTestFavoriteService()
		err := svc.AddItem(AddFavoriteItemInput{UserID: 0, FolderID: 1, ProductID: 1})
		mustBeBizError(t, err, http.StatusBadRequest, "用户 ID 不合法")
	})

	t.Run("invalid_folder_id", func(t *testing.T) {
		svc, _, _ := newTestFavoriteService()
		err := svc.AddItem(AddFavoriteItemInput{UserID: 1, FolderID: 0, ProductID: 1})
		mustBeBizError(t, err, http.StatusBadRequest, "收藏夹 ID 不合法")
	})

	t.Run("invalid_product_id", func(t *testing.T) {
		svc, _, _ := newTestFavoriteService()
		err := svc.AddItem(AddFavoriteItemInput{UserID: 1, FolderID: 1, ProductID: 0})
		mustBeBizError(t, err, http.StatusBadRequest, "商品 ID 不合法")
	})

	t.Run("folder_not_found", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		favDB.err = gorm.ErrRecordNotFound
		err := svc.AddItem(AddFavoriteItemInput{UserID: 1, FolderID: 99, ProductID: 1})
		mustBeBizError(t, err, http.StatusNotFound, "收藏夹不存在")
	})

	t.Run("get_folder_db_error", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		dbErr := errors.New("db timeout")
		favDB.err = dbErr
		err := svc.AddItem(AddFavoriteItemInput{UserID: 1, FolderID: 1, ProductID: 1})
		assert.Equal(t, dbErr, err)
	})

	t.Run("product_not_found", func(t *testing.T) {
		svc, favDB, prodDB := newTestFavoriteService()
		favDB.folder = &model.FavoriteFolder{}
		prodDB.err = gorm.ErrRecordNotFound

		err := svc.AddItem(AddFavoriteItemInput{UserID: 1, FolderID: 1, ProductID: 99})
		mustBeBizError(t, err, http.StatusNotFound, "商品不存在或已下架")
	})

	t.Run("get_product_db_error", func(t *testing.T) {
		svc, favDB, prodDB := newTestFavoriteService()
		favDB.folder = &model.FavoriteFolder{}
		dbErr := errors.New("product db down")
		prodDB.err = dbErr

		err := svc.AddItem(AddFavoriteItemInput{UserID: 1, FolderID: 1, ProductID: 1})
		assert.Equal(t, dbErr, err)
	})

	t.Run("already_favorited", func(t *testing.T) {
		svc, favDB, prodDB := newTestFavoriteService()
		favDB.folder = &model.FavoriteFolder{}
		prodDB.product = &model.Product{}
		favDB.item = &model.FavoriteItem{}

		err := svc.AddItem(AddFavoriteItemInput{UserID: 1, FolderID: 1, ProductID: 1})
		mustBeBizError(t, err, http.StatusBadRequest, "该商品已收藏")
	})

	t.Run("get_item_db_error", func(t *testing.T) {
		svc, favDB, prodDB := newTestFavoriteService()
		favDB.getFolderByIDFunc = func(userID, folderID uint) (*model.FavoriteFolder, error) {
			return &model.FavoriteFolder{}, nil
		}
		prodDB.product = &model.Product{}
		dbErr := errors.New("db error")
		favDB.getItemFunc = func(userID, folderID, productID uint) (*model.FavoriteItem, error) {
			return nil, dbErr
		}

		err := svc.AddItem(AddFavoriteItemInput{UserID: 1, FolderID: 1, ProductID: 1})
		assert.Equal(t, dbErr, err)
	})

	t.Run("success", func(t *testing.T) {
		svc, favDB, prodDB := newTestFavoriteService()
		// GetFolderByID succeeds via per-method override (so m.err can stay for GetItem)
		favDB.getFolderByIDFunc = func(userID, folderID uint) (*model.FavoriteFolder, error) {
			return &model.FavoriteFolder{}, nil
		}
		prodDB.product = &model.Product{}
		// GetItem: not found (item=nil + ErrRecordNotFound)
		favDB.item = nil
		favDB.err = gorm.ErrRecordNotFound
		// AddItem: succeed (override because m.err is still ErrRecordNotFound)
		favDB.addItemFunc = func(item *model.FavoriteItem) error {
			return nil
		}

		err := svc.AddItem(AddFavoriteItemInput{UserID: 5, FolderID: 3, ProductID: 7})
		assert.NoError(t, err)

		item := favDB.lastAddItemArg
		assert.NotNil(t, item)
		assert.Equal(t, uint(5), item.UserID)
		assert.Equal(t, uint(3), item.FolderID)
		assert.Equal(t, uint(7), item.ProductID)
	})

	t.Run("add_item_db_error", func(t *testing.T) {
		svc, favDB, prodDB := newTestFavoriteService()
		favDB.getFolderByIDFunc = func(userID, folderID uint) (*model.FavoriteFolder, error) {
			return &model.FavoriteFolder{}, nil
		}
		prodDB.product = &model.Product{}
		// GetItem: not found
		favDB.item = nil
		favDB.err = gorm.ErrRecordNotFound
		// AddItem: fails — use per-method override
		addErr := errors.New("add item failed")
		favDB.addItemFunc = func(item *model.FavoriteItem) error {
			return addErr
		}

		err := svc.AddItem(AddFavoriteItemInput{UserID: 1, FolderID: 1, ProductID: 1})
		assert.Equal(t, addErr, err)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// DeleteItem
// ═══════════════════════════════════════════════════════════════════════════════

func TestFavoriteService_DeleteItem(t *testing.T) {

	t.Run("invalid_user_id", func(t *testing.T) {
		svc, _, _ := newTestFavoriteService()
		err := svc.DeleteItem(DeleteFavoriteItemInput{UserID: 0, FolderID: 1, ProductID: 1})
		mustBeBizError(t, err, http.StatusBadRequest, "用户 ID 不合法")
	})

	t.Run("invalid_folder_id", func(t *testing.T) {
		svc, _, _ := newTestFavoriteService()
		err := svc.DeleteItem(DeleteFavoriteItemInput{UserID: 1, FolderID: 0, ProductID: 1})
		mustBeBizError(t, err, http.StatusBadRequest, "收藏夹 ID 不合法")
	})

	t.Run("invalid_product_id", func(t *testing.T) {
		svc, _, _ := newTestFavoriteService()
		err := svc.DeleteItem(DeleteFavoriteItemInput{UserID: 1, FolderID: 1, ProductID: 0})
		mustBeBizError(t, err, http.StatusBadRequest, "商品 ID 不合法")
	})

	t.Run("not_found", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		favDB.rowsAffected = 0
		err := svc.DeleteItem(DeleteFavoriteItemInput{UserID: 1, FolderID: 1, ProductID: 99})
		mustBeBizError(t, err, http.StatusNotFound, "收藏记录不存在")
	})

	t.Run("success", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		favDB.rowsAffected = 1
		err := svc.DeleteItem(DeleteFavoriteItemInput{UserID: 1, FolderID: 2, ProductID: 3})
		assert.NoError(t, err)
		assert.Equal(t, uint(1), favDB.lastDeleteItem.userID)
		assert.Equal(t, uint(2), favDB.lastDeleteItem.folderID)
		assert.Equal(t, uint(3), favDB.lastDeleteItem.productID)
	})

	t.Run("db_error", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		dbErr := errors.New("delete failed")
		favDB.err = dbErr
		err := svc.DeleteItem(DeleteFavoriteItemInput{UserID: 1, FolderID: 1, ProductID: 1})
		assert.Equal(t, dbErr, err)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// ListItems
// ═══════════════════════════════════════════════════════════════════════════════

func TestFavoriteService_ListItems(t *testing.T) {

	t.Run("invalid_user_id", func(t *testing.T) {
		svc, _, _ := newTestFavoriteService()
		result, err := svc.ListItems(ListFavoriteItemsInput{UserID: 0, FolderID: 1})
		assert.Nil(t, result)
		mustBeBizError(t, err, http.StatusBadRequest, "用户 ID 不合法")
	})

	t.Run("invalid_folder_id", func(t *testing.T) {
		svc, _, _ := newTestFavoriteService()
		result, err := svc.ListItems(ListFavoriteItemsInput{UserID: 1, FolderID: 0})
		assert.Nil(t, result)
		mustBeBizError(t, err, http.StatusBadRequest, "收藏夹 ID 不合法")
	})

	t.Run("defaults_page_and_page_size", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		favDB.folder = &model.FavoriteFolder{}
		favDB.items = []*model.FavoriteItem{}

		result, err := svc.ListItems(ListFavoriteItemsInput{
			UserID: 1, FolderID: 1, Page: 0, PageSize: 0,
		})
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 1, favDB.lastListItemsQuery.Page)
		assert.Equal(t, 20, favDB.lastListItemsQuery.PageSize)
		assert.Equal(t, 1, result.Page)
		assert.Equal(t, 20, result.PageSize)
	})

	t.Run("caps_page_size", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		favDB.folder = &model.FavoriteFolder{}
		favDB.items = []*model.FavoriteItem{}

		result, err := svc.ListItems(ListFavoriteItemsInput{
			UserID: 1, FolderID: 1, Page: 1, PageSize: 200,
		})
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 100, favDB.lastListItemsQuery.PageSize)
		assert.Equal(t, 100, result.PageSize)
	})

	t.Run("folder_not_found", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		favDB.err = gorm.ErrRecordNotFound
		result, err := svc.ListItems(ListFavoriteItemsInput{UserID: 1, FolderID: 99, Page: 1, PageSize: 10})
		assert.Nil(t, result)
		mustBeBizError(t, err, http.StatusNotFound, "收藏夹不存在")
	})

	t.Run("get_folder_db_error", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		dbErr := errors.New("db timeout")
		favDB.err = dbErr
		result, err := svc.ListItems(ListFavoriteItemsInput{UserID: 1, FolderID: 1, Page: 1, PageSize: 10})
		assert.Nil(t, result)
		assert.Equal(t, dbErr, err)
	})

	t.Run("list_items_db_error", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		favDB.folder = &model.FavoriteFolder{}
		dbErr := errors.New("list failed")
		favDB.err = dbErr
		result, err := svc.ListItems(ListFavoriteItemsInput{UserID: 1, FolderID: 1, Page: 1, PageSize: 10})
		assert.Nil(t, result)
		assert.Equal(t, dbErr, err)
	})

	t.Run("success", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		favDB.folder = &model.FavoriteFolder{}
		wantItems := []*model.FavoriteItem{
			{UserID: 1, FolderID: 1, ProductID: 10},
			{UserID: 1, FolderID: 1, ProductID: 20},
		}
		favDB.items = wantItems
		favDB.total = 15

		result, err := svc.ListItems(ListFavoriteItemsInput{
			UserID: 1, FolderID: 1, Page: 2, PageSize: 5,
		})
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, wantItems, result.Items)
		assert.Equal(t, int64(15), result.Total)
		assert.Equal(t, 2, result.Page)
		assert.Equal(t, 5, result.PageSize)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// IsProductFavorited
// ═══════════════════════════════════════════════════════════════════════════════

func TestFavoriteService_IsProductFavorited(t *testing.T) {

	t.Run("invalid_user_id", func(t *testing.T) {
		svc, _, _ := newTestFavoriteService()
		ok, err := svc.IsProductFavorited(0, 1)
		assert.False(t, ok)
		mustBeBizError(t, err, http.StatusBadRequest, "用户 ID 不合法")
	})

	t.Run("invalid_product_id", func(t *testing.T) {
		svc, _, _ := newTestFavoriteService()
		ok, err := svc.IsProductFavorited(1, 0)
		assert.False(t, ok)
		mustBeBizError(t, err, http.StatusBadRequest, "商品 ID 不合法")
	})

	t.Run("success_true", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		favDB.isFavorited = true
		ok, err := svc.IsProductFavorited(1, 10)
		assert.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("success_false", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		favDB.isFavorited = false
		ok, err := svc.IsProductFavorited(1, 10)
		assert.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("db_error", func(t *testing.T) {
		svc, favDB, _ := newTestFavoriteService()
		dbErr := errors.New("db down")
		favDB.err = dbErr
		ok, err := svc.IsProductFavorited(1, 10)
		assert.False(t, ok)
		assert.Equal(t, dbErr, err)
	})
}
