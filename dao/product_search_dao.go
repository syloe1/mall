package dao

import "mall/model"

type ProductSearchDB interface {
	//用户端搜索商品（商城搜索框）
	SearchPublic(query ProductListQuery) ([]model.Product, int64, error)
	//把商品写入搜索库（建立索引）
	IndexProduct(product *model.Product) error
	//全量重建索引：批量写入全部商品（对账/补偿用）
	ReplaceAllProducts(products []model.Product) (int, error)
	//从搜索库中删除商品
	DeleteProduct(id uint) error
}
