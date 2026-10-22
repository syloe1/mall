package service

import (
	"errors"
	"fmt"
	"mall/dao"
	"mall/model"
	"mall/pkg/core"
	"net/http"
	"time"

	"gorm.io/gorm"
)

type CreateProductInput struct {
	Name        string
	PriceCents  int64
	Description string
	Stock       int
	CategoryID  uint
	Status      string
	Images      []string
}

type UpdateProductInput struct {
	ID          uint
	Name        string
	PriceCents  int64
	Description string
	Stock       int
	CategoryID  uint
	Status      string
	Images      []string
}

type ListAdminProductsInput struct {
	Page       int
	PageSize   int
	Status     string // 可筛选：上架/下架
	CategoryID uint   // 按分类筛选
	Keyword    string // 搜索关键词
}

type ListAdminProductsResult struct {
	Items    []model.Product `json:"items"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

type ListPublicProductsInput struct {
	Page       int
	PageSize   int
	CategoryID uint
	Keyword    string
}

type ListPublicProductsResult struct {
	Items    []model.Product `json:"items"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// ES搜索
type SearchPublicProductsInput struct {
	Page       int
	PageSize   int
	CategoryID uint
	Keyword    string
}

type SearchPublicProductsResult struct {
	Items    []model.Product `json:"items"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

type ProductService struct {
	db       dao.ProductDB
	searchDB dao.ProductSearchDB
}

func NewProductService(db dao.ProductDB, searchDB dao.ProductSearchDB) *ProductService {
	return &ProductService{
		db:       db,
		searchDB: searchDB,
	}
}

func (p *ProductService) Create(input CreateProductInput) error {
	if input.Status == "" {
		input.Status = model.ProductDraft
	}

	switch input.Status {
	case model.ProductDraft, model.ProductOnSale, model.ProductOffSale:
	default:
		return errors.New("商品状态不合法")
	}

	if input.Status == model.ProductOnSale && len(input.Images) == 0 {
		return fmt.Errorf("on_sale product requires at least one image")
	}

	images := make([]model.ProductImage, 0, len(input.Images))
	for i, url := range input.Images {
		images = append(images, model.ProductImage{
			URL:       url,
			SortOrder: i + 1,
		})
	}

	product := &model.Product{
		Name:        input.Name,
		PriceCents:  input.PriceCents,
		Description: input.Description,
		Stock:       input.Stock,
		CategoryID:  input.CategoryID,
		Status:      input.Status,
		SaleCount:   0,
		Images:      images,
	}

	return p.db.Transaction(func(txProductDB dao.ProductDB, txOutboxDB dao.ProductOutboxDB) error {
		if err := txProductDB.Create(product); err != nil {
			return err
		}
		// 同时插入一条【创建事件】到发件箱表（Outbox）
		return txOutboxDB.Create(newProductOutbox(model.ProductEventCreated, product.ID))
	})
}

func (s *ProductService) GetPublic(id uint) (*model.Product, error) {
	if id == 0 {
		return nil, core.NewBizError(http.StatusBadRequest, "商品 ID 不合法")
	}
	// 查询【公开/在售】商品（
	product, err := s.db.GetPublicByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.NewBizError(http.StatusNotFound, "商品不存在或已下架")
		}
		return nil, err
	}
	return product, nil
}

func (s *ProductService) ListPublic(input ListPublicProductsInput) (*ListPublicProductsResult, error) {
	if input.Page <= 0 {
		input.Page = 1
	}
	if input.PageSize <= 0 {
		input.PageSize = 20
	}
	if input.PageSize > 100 {
		input.PageSize = 100
	}

	products, total, err := s.db.ListPublic(dao.ProductListQuery{
		Page:       input.Page,
		PageSize:   input.PageSize,
		CategoryID: input.CategoryID,
		Keyword:    input.Keyword,
	})
	if err != nil {
		return nil, err
	}

	return &ListPublicProductsResult{
		Items:    products,
		Total:    total,
		Page:     input.Page,
		PageSize: input.PageSize,
	}, nil
}

func (s *ProductService) ListAdmin(input ListAdminProductsInput) (*ListAdminProductsResult, error) {
	if input.Page <= 0 {
		input.Page = 1
	}
	if input.PageSize <= 0 {
		input.PageSize = 20
	}
	if input.PageSize > 100 {
		input.PageSize = 100
	}
	if input.Status != "" {
		switch input.Status {
		case model.ProductDraft, model.ProductOnSale, model.ProductOffSale, model.ProductSoldOut:
		default:
			return nil, errors.New("商品状态不合法")
		}
	}

	products, total, err := s.db.ListAdmin(dao.ProductListQuery{
		Page:       input.Page,
		PageSize:   input.PageSize,
		Status:     input.Status,
		CategoryID: input.CategoryID,
		Keyword:    input.Keyword,
	})
	if err != nil {
		return nil, err
	}

	return &ListAdminProductsResult{
		Items:    products,
		Total:    total,
		Page:     input.Page,
		PageSize: input.PageSize,
	}, nil
}

func (s *ProductService) UpdateProduct(input UpdateProductInput) error {
	if input.ID == 0 {
		return errors.New("商品 ID 不合法")
	}
	product, err := s.db.GetProductById(input.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("商品不存在或已下架")
		}
		return err
	}
	if input.Name != "" {
		product.Name = input.Name
	}
	if input.PriceCents > 0 {
		product.PriceCents = input.PriceCents
	}
	if input.Description != "" {
		product.Description = input.Description
	}
	if input.Stock >= 0 {
		product.Stock = input.Stock
	}
	if input.CategoryID > 0 {
		product.CategoryID = input.CategoryID
	}
	if input.Status != "" {
		product.Status = input.Status
	}

	replaceImages := input.Status == model.ProductOnSale && len(input.Images) > 0
	if replaceImages {
		images := make([]model.ProductImage, 0, len(input.Images))
		for i, url := range input.Images {
			images = append(images, model.ProductImage{
				URL:       url,
				SortOrder: i + 1,
			})
		}
		product.Images = images
	}

	return s.db.Transaction(func(txProductDB dao.ProductDB, txOutboxDB dao.ProductOutboxDB) error {
		if replaceImages {
			if err := txProductDB.DeleteImagesByProductID(product.ID); err != nil {
				return err
			}
		}
		if err := txProductDB.Update(product); err != nil {
			return err
		}
		if replaceImages {
			for _, image := range product.Images {
				next := image
				next.ProductID = product.ID
				if err := txProductDB.AddImage(&next); err != nil {
					return err
				}
			}
		}
		//  写入发件箱表：商品已更新（保证同步 ES）
		return txOutboxDB.Create(newProductOutbox(model.ProductEventUpdated, product.ID))
	})
}

func (s *ProductService) DeleteProduct(id uint) error {
	if id == 0 {
		return errors.New("商品 ID 不合法")
	}
	if _, err := s.db.GetProductById(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("商品不存在或已下架")
		}
		return err
	}

	return s.db.Transaction(func(txProductDB dao.ProductDB, txOutboxDB dao.ProductOutboxDB) error {
		if err := txProductDB.DeleteProductByID(id); err != nil {
			return err
		}
		//插入一条【删除事件】到发件箱表
		// 保证：商品删了，ES 也一定会同步删除

		return txOutboxDB.Create(newProductOutbox(model.ProductEventDeleted, id))
	})
}

// ReindexAll 全量重建搜索索引（数据补偿/对账）：
// 分页拉取 MySQL 全部商品，批量覆盖写入 Meilisearch。返回重建条数。
func (s *ProductService) ReindexAll() (int, error) {
	if s.searchDB == nil {
		return 0, errors.New("搜索服务未初始化")
	}
	const size = 100
	all := make([]model.Product, 0)
	page := 1
	for {
		products, total, err := s.db.ListAdmin(dao.ProductListQuery{Page: page, PageSize: size})
		if err != nil {
			return 0, err
		}
		all = append(all, products...)
		if int64(len(all)) >= total || len(products) == 0 {
			break
		}
		page++
	}
	return s.searchDB.ReplaceAllProducts(all)
}

func (s *ProductService) SearchPublic(input SearchPublicProductsInput) (*SearchPublicProductsResult, error) {
	if input.Page <= 0 {
		input.Page = 1
	}
	if input.PageSize <= 0 {
		input.PageSize = 20
	}
	if input.PageSize > 100 {
		input.PageSize = 100
	}
	//	 如果 ES 未初始化，直接报错（不能用）
	if s.searchDB == nil {
		return nil, errors.New("搜索服务未初始化")
	}
	//从 ES 搜索商品（全文搜索、关键词、分类）
	products, total, err := s.searchDB.SearchPublic(dao.ProductListQuery{
		Page:       input.Page,
		PageSize:   input.PageSize,
		CategoryID: input.CategoryID,
		Keyword:    input.Keyword,
	})
	if err != nil {
		return nil, err
	}

	return &SearchPublicProductsResult{
		Items:    products,
		Total:    total,
		Page:     input.Page,
		PageSize: input.PageSize,
	}, nil
}

func newProductOutbox(eventType string, productID uint) *model.ProductOutbox {
	return &model.ProductOutbox{
		EventID:   fmt.Sprintf("%s-%d-%d", eventType, productID, time.Now().UnixNano()),
		EventType: eventType,
		ProductID: productID,
		Status:    model.ProductOutboxPending,
	}
}
