package service

import (
	"errors"
	"mall/dao"
	"mall/model"
	"mall/pkg/core"
	"net/http"

	"gorm.io/gorm"
)

type CartService struct {
	db        dao.CartDB
	productDB dao.ProductDB
}

func NewCartService(db dao.CartDB, productDB dao.ProductDB) *CartService {
	return &CartService{db: db, productDB: productDB}
}

type AddCartItemInput struct {
	UserID    uint
	ProductID uint
	Quantity  int
}

type UpdateCartItemInput struct {
	UserID    uint
	ProductID uint
	Quantity  int
}

type DeleteCartItemInput struct {
	UserID    uint
	ProductID uint
}

type SetCartItemCheckedInput struct {
	UserID    uint
	ProductID uint
	Checked   bool
}

type SetCartAllCheckedInput struct {
	UserID  uint
	Checked bool
}

func (s *CartService) AddItem(input AddCartItemInput) error {
	if input.UserID == 0 {
		return core.NewBizError(http.StatusBadRequest, "用户 ID 不合法")
	}
	if input.ProductID == 0 {
		return core.NewBizError(http.StatusBadRequest, "商品 ID 不合法")
	}
	if input.Quantity <= 0 {
		return core.NewBizError(http.StatusBadRequest, "数量必须大于 0")
	}

	product, err := s.productDB.GetProductById(input.ProductID)
	if err != nil {
		// 商品不存在
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return core.NewBizError(http.StatusNotFound, "商品不存在或已下架")
		}
		return err
	}

	// 商品必须是【上架】状态才能买
	if product.Status != model.ProductOnSale {
		return core.NewBizError(http.StatusBadRequest, "商品已下架")
	}
	// 库存必须足够（不能买比库存多的数量）
	if product.Stock < input.Quantity {
		return core.NewBizError(http.StatusBadRequest, "库存不足")
	}

	item, err := s.db.GetItem(input.UserID, input.ProductID)
	//gorm.ErrRecordNotFound 正常购物没有商品
	if err == nil {
		//购物车已有该商品 → 累加数量
		newQuantity := item.Quantity + input.Quantity
		if newQuantity > product.Stock {
			return core.NewBizError(http.StatusBadRequest, "库存不足")
		}
		item.Quantity = newQuantity
		item.Checked = true
		return s.db.UpdateItem(item)
	}
	// 如果错误是“未找到”，说明购物车还没这个商品
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		//其他Error
		return err
	}
	// 直接创建新的购物车记录
	return s.db.CreateItem(&model.CartItem{
		UserID:    input.UserID,
		ProductID: input.ProductID,
		Quantity:  input.Quantity,
		Checked:   true,
	})
}

func (s *CartService) ListItems(userID uint) ([]model.CartItem, error) {
	if userID == 0 {
		return nil, core.NewBizError(http.StatusBadRequest, "用户 ID 不合法")
	}
	return s.db.ListItems(userID)
}

func (s *CartService) UpdateItem(input UpdateCartItemInput) error {
	if input.UserID == 0 || input.ProductID == 0 || input.Quantity <= 0 {
		return core.NewBizError(http.StatusBadRequest, "请求参数不合法")
	}

	item, err := s.db.GetItem(input.UserID, input.ProductID)
	if err != nil {
		//购物车商品不存在
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return core.NewBizError(http.StatusNotFound, "购物车商品不存在")
		}
		return err
	}

	product, err := s.productDB.GetProductById(input.ProductID)
	if err != nil {
		return err
	}
	// 要修改的数量 > 库存 → 不允许
	if input.Quantity > product.Stock {
		return core.NewBizError(http.StatusBadRequest, "库存不足")
	}

	item.Quantity = input.Quantity
	return s.db.UpdateItem(item)
}

func (s *CartService) DeleteItem(input DeleteCartItemInput) error {
	/*
		delete from cart where user_id = ? and product_id = ?
	*/
	rowsAffected, err := s.db.DeleteItem(input.UserID, input.ProductID)
	//数据库报错，直接返回
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return core.NewBizError(http.StatusNotFound, "购物车商品不存在")
	}
	return nil
}

func (s *CartService) Clear(userID uint) error {
	if userID == 0 {
		return core.NewBizError(http.StatusBadRequest, "用户 ID 不合法")
	}
	return s.db.Clear(userID)
}

func (s *CartService) SetItemChecked(input SetCartItemCheckedInput) error {
	item, err := s.db.GetItem(input.UserID, input.ProductID)
	if err != nil {
		//不存在
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return core.NewBizError(http.StatusNotFound, "购物车商品不存在")
		}
		//other error
		return err
	}
	// 如果用户是 勾选商品（不是取消）
	if input.Checked {
		// 查商品信息
		product, err := s.productDB.GetProductById(input.ProductID)
		if err != nil {
			return err
		}
		//必须上架的， 必须足够库存
		if product.Status != model.ProductOnSale || product.Stock < item.Quantity {
			return core.NewBizError(http.StatusBadRequest, "该商品不可勾选")
		}
	}

	rowsAffected, err := s.db.SetItemChecked(input.UserID, input.ProductID, input.Checked)
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return core.NewBizError(http.StatusNotFound, "购物车商品不存在")
	}
	return nil
}

func (s *CartService) SetAllChecked(input SetCartAllCheckedInput) error {
	if input.UserID == 0 {
		return core.NewBizError(http.StatusBadRequest, "用户 ID 不合法")
	}
	return s.db.SetAllChecked(input.UserID, input.Checked)
}

func (s *CartService) ListCheckedItems(userID uint) ([]model.CartItem, error) {
	if userID == 0 {
		return nil, core.NewBizError(http.StatusBadRequest, "用户 ID 不合法")
	}
	return s.db.ListCheckedItems(userID)
}
