package mock

import (
	"mall/dao"
	"mall/model"

	"github.com/stretchr/testify/mock"
)

type MockFavoriteDB struct {
	mock.Mock
}

func (m *MockFavoriteDB) CreateFolder(folder *model.FavoriteFolder) error {
	return m.Called(folder).Error(0)
}

func (m *MockFavoriteDB) GetFolderByID(userID uint, folderID uint) (*model.FavoriteFolder, error) {
	args := m.Called(userID, folderID)
	f, ok := args.Get(0).(*model.FavoriteFolder)
	if !ok {
		return nil, args.Error(1)
	}
	return f, args.Error(1)
}

func (m *MockFavoriteDB) ListFolders(userID uint) ([]*model.FavoriteFolder, error) {
	args := m.Called(userID)
	list, ok := args.Get(0).([]*model.FavoriteFolder)
	if !ok {
		return nil, args.Error(1)
	}
	return list, args.Error(1)
}

func (m *MockFavoriteDB) UpdateFolder(folder *model.FavoriteFolder) error {
	return m.Called(folder).Error(0)
}

func (m *MockFavoriteDB) DeleteFolder(userID uint, folderID uint) error {
	return m.Called(userID, folderID).Error(0)
}

func (m *MockFavoriteDB) AddItem(item *model.FavoriteItem) error {
	return m.Called(item).Error(0)
}

func (m *MockFavoriteDB) DeleteItem(userID uint, folderID uint, productID uint) (int64, error) {
	args := m.Called(userID, folderID, productID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockFavoriteDB) GetItem(userID uint, folderID uint, productID uint) (*model.FavoriteItem, error) {
	args := m.Called(userID, folderID, productID)
	item, ok := args.Get(0).(*model.FavoriteItem)
	if !ok {
		return nil, args.Error(1)
	}
	return item, args.Error(1)
}

func (m *MockFavoriteDB) ListItems(query dao.FavoriteItemQuery) ([]*model.FavoriteItem, int64, error) {
	args := m.Called(query)
	list, ok := args.Get(0).([]*model.FavoriteItem)
	if !ok {
		return nil, 0, args.Error(2)
	}
	return list, args.Get(1).(int64), args.Error(2)
}

func (m *MockFavoriteDB) IsProductFavorited(userID uint, productID uint) (bool, error) {
	args := m.Called(userID, productID)
	return args.Get(0).(bool), args.Error(1)
}
