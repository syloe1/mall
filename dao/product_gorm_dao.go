package dao

import (
	"mall/model"
	"strings"

	"gorm.io/gorm"
)

type GormProductDB struct {
	DB *gorm.DB
}

func (g *GormProductDB) Transaction(fn func(txProductDB ProductDB, txOutboxDB ProductOutboxDB) error) error {
	/*
		BEGIN
		执行扣库存， 更新商品等操作
		COMMIT
	*/
	return g.DB.Transaction(func(tx *gorm.DB) error {
		return fn(
			&GormProductDB{DB: tx},
			&GormProductOutboxDB{DB: tx},
		)
	})
}

func (g *GormProductDB) Create(product *model.Product) error {
	//INSERT INTO products () values ()
	return g.DB.Create(product).Error
}
func (g *GormProductDB) GetProductById(id uint) (*model.Product, error) {
	var product model.Product
	/*
		select * from products where id = ? limit 1

		select * from product_images
		where product_id = ? order by sort_order asc
	*/
	if err := g.DB.Preload("Images", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Where("id = ?", id).First(&product).Error; err != nil {
		return nil, err
	}
	return &product, nil
}
func (g *GormProductDB) GetPublicByID(id uint) (*model.Product, error) {
	var product model.Product
	if err := g.DB.Preload("Images", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Where("id = ? AND status = ?", id, model.ProductOnSale).First(&product).Error; err != nil {
		return nil, err
	}
	return &product, nil
}
func (g *GormProductDB) ListAdmin(query ProductListQuery) ([]model.Product, int64, error) {
	var products []model.Product
	var total int64

	status := strings.TrimSpace(query.Status)
	keyword := strings.TrimSpace(query.Keyword)

	base := g.DB.Model(&model.Product{})
	if status != "" {
		base = base.Where("status = ?", status)
	}
	if query.CategoryID > 0 {
		base = base.Where("category_id = ?", query.CategoryID)
	}
	if keyword != "" {
		base = base.Where("name LIKE ?", "%"+keyword+"%")
	}

	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (query.Page - 1) * query.PageSize
	err := base.Session(&gorm.Session{}).Preload("Images", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Order("created_at DESC").Offset(offset).Limit(query.PageSize).Find(&products).Error
	if err != nil {
		return nil, 0, err
	}

	return products, total, nil
}
func (g *GormProductDB) ListPublic(query ProductListQuery) ([]model.Product, int64, error) {
	var products []model.Product
	var total int64

	keyword := strings.TrimSpace(query.Keyword)

	base := g.DB.Model(&model.Product{}).Where("status = ?", model.ProductOnSale)
	if query.CategoryID > 0 {
		base = base.Where("category_id = ?", query.CategoryID)
	}
	if keyword != "" {
		base = base.Where("name LIKE ?", "%"+keyword+"%")
	}

	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (query.Page - 1) * query.PageSize
	err := base.Session(&gorm.Session{}).Preload("Images", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Order("created_at DESC").Offset(offset).Limit(query.PageSize).Find(&products).Error
	if err != nil {
		return nil, 0, err
	}

	return products, total, nil
}
func (g *GormProductDB) Update(product *model.Product) error {
	//UPDATE products set ... where id = ?
	return g.DB.Save(product).Error
}
func (g *GormProductDB) AddImage(image *model.ProductImage) error {
	return g.DB.Create(image).Error
}
func (g *GormProductDB) DeleteImage(productID uint, imageID uint) (int64, error) {
	/*
		delete from product_images
		where id = ？ and product_id = ?
	*/
	result := g.DB.Delete(&model.ProductImage{}, "id = ? AND product_id = ?", imageID, productID)
	return result.RowsAffected, result.Error
}
func (g *GormProductDB) DeleteImagesByProductID(productID uint) error {
	//delete from product_images where product_id =  ?
	return g.DB.Delete(&model.ProductImage{}, "product_id = ?", productID).Error
}
func (g *GormProductDB) DeleteProductByID(productID uint) error {
	/*
		BEGIN;
		DELETE FROM product_images WHERE product_id = ?;
		DELETE FROM products WHERE id = ?;
		COMMIT;
	*/
	return g.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.ProductImage{}, "product_id = ?", productID).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Product{}, productID).Error
	})
}
func (g *GormProductDB) SearchPublic(query ProductListQuery) ([]model.Product, int64, error) {
	var products []model.Product
	var total int64
	db := g.DB.Model(&model.Product{}).
		Where("status = ?", model.ProductOnSale)
	if query.CategoryID > 0 {
		db = db.Where("category_id = ?", query.CategoryID)
	}
	if query.Keyword != "" {
		db = db.Where("name LIKE ?", "%"+query.Keyword+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (query.Page - 1) * query.PageSize
	if err := db.Preload("Images", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Order("created_at DESC").Offset(offset).Limit(query.PageSize).Find(&products).Error; err != nil {
		return nil, 0, err
	}
	return products, total, nil
}
func (g *GormProductDB) DecreaseStock(productID uint, quantity int) (int64, error) {
	/*
		update products set stock = stock - ? where id = ? and stock >= ?
	*/
	result := g.DB.Model(&model.Product{}).
		Where("id = ? AND stock >= ?", productID, quantity).
		UpdateColumn("stock", gorm.Expr("stock - ?", quantity))
	return result.RowsAffected, result.Error
}
