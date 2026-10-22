package dao

import (
	"mall/model"
)

type ProductListQuery struct {
	Page       int    // 页码
	PageSize   int    // 每页条数
	Status     string // 商品状态：上架/下架
	CategoryID uint   // 分类ID
	Keyword    string // 搜索关键词
}

type ProductDB interface {
	Create(product *model.Product) error
	//管理员 / 后台根据 ID 查商品
	GetProductById(id uint) (*model.Product, error)
	//用户端查询商品（必须是上架状态）
	GetPublicByID(id uint) (*model.Product, error)
	//后台商品管理列表
	ListAdmin(query ProductListQuery) ([]model.Product, int64, error)
	//用户端商城首页、分类页商品列表
	ListPublic(query ProductListQuery) ([]model.Product, int64, error)
	//修改商品信息（名称、价格、库存、状态）
	Update(product *model.Product) error
	AddImage(image *model.ProductImage) error
	//删除单张图片
	DeleteImage(productID uint, imageID uint) (int64, error)
	//删除某个商品的所有图片
	DeleteImagesByProductID(productID uint) error
	//删除商品
	DeleteProductByID(productID uint) error
	//商城搜索商品（按关键词搜索）
	SearchPublic(query ProductListQuery) ([]model.Product, int64, error)
	//扣减商品库存（下单时必须调用）
	DecreaseStock(productId uint, quantity int) (int64, error)
	Transaction(fn func(txProductDB ProductDB, txOutboxDB ProductOutboxDB) error) error
}
