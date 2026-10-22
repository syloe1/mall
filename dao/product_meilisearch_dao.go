package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"mall/model"

	"github.com/meilisearch/meilisearch-go"
)

// MeiliProductDB 基于 Meilisearch 的商品搜索实现
// 和原来的 ESProductDB 平级：实现同一个 ProductSearchDB 接口
// 上层（service/handler）完全不用改，想切回 ES 时再写一个实现即可
type MeiliProductDB struct {
	client meilisearch.ServiceManager // Meilisearch 客户端
	index  string                     // 索引名（相当于 ES 的 index、MySQL 的表）

	mu    sync.Mutex // 保护 ready，保证初始化只跑一次
	ready bool       // 索引和字段配置都就绪了
}

// NewMeiliProductDB 创建 Meilisearch 搜索实现
// host: http://127.0.0.1:7700  index: 商品索引名（默认 mall_products）
func NewMeiliProductDB(host, apiKey, index string) *MeiliProductDB {
	return &MeiliProductDB{
		client: meilisearch.New(host, meilisearch.WithAPIKey(apiKey)),
		index:  index,
	}
}

// 商品搜索：全文搜索 + 状态过滤 + 分类过滤 + 时间倒序
func (d *MeiliProductDB) SearchPublic(query ProductListQuery) ([]model.Product, int64, error) {
	if err := d.ensureReady(); err != nil {
		return nil, 0, err
	}

	// 过滤条件：只搜在售商品，可选按分类过滤
	filters := []string{fmt.Sprintf("status = %q", model.ProductOnSale)}
	if query.CategoryID > 0 {
		filters = append(filters, fmt.Sprintf("category_id = %d", query.CategoryID))
	}

	req := &meilisearch.SearchRequest{
		Limit:  int64(query.PageSize),
		Offset: int64((query.Page - 1) * query.PageSize),
		Filter: strings.Join(filters, " AND "),
		Sort:   []string{"created_at:desc"},
	}
	resp, err := d.client.Index(d.index).Search(strings.TrimSpace(query.Keyword), req)
	if err != nil {
		return nil, 0, fmt.Errorf("meilisearch search failed: %w", err)
	}

	products := make([]model.Product, 0, len(resp.Hits))
	for _, hit := range resp.Hits {
		b, err := json.Marshal(hit)
		if err != nil {
			continue
		}
		var doc meiliProductDoc
		if err := json.Unmarshal(b, &doc); err != nil {
			continue
		}
		products = append(products, doc.toModel())
	}
	return products, resp.EstimatedTotalHits, nil
}

// 建立/更新商品索引（商品新增、修改时调用，按 id 覆盖写，幂等）
func (d *MeiliProductDB) IndexProduct(product *model.Product) error {
	if err := d.ensureReady(); err != nil {
		return err
	}
	if product == nil || product.ID == 0 {
		return fmt.Errorf("invalid product")
	}

	task, err := d.client.Index(d.index).AddDocuments([]*meiliProductDoc{newMeiliProductDoc(product)}, "id")
	if err != nil {
		return fmt.Errorf("meilisearch index product failed: %w", err)
	}
	return d.waitTask(task)
}

// 全量重建索引：把传入的全部商品批量覆盖写入（按 id 幂等）
func (d *MeiliProductDB) ReplaceAllProducts(products []model.Product) (int, error) {
	if err := d.ensureReady(); err != nil {
		return 0, err
	}
	idx := d.client.Index(d.index)
	const batch = 100
	indexed := 0
	for start := 0; start < len(products); start += batch {
		end := start + batch
		if end > len(products) {
			end = len(products)
		}
		docs := make([]*meiliProductDoc, 0, end-start)
		for i := start; i < end; i++ {
			docs = append(docs, newMeiliProductDoc(&products[i]))
		}
		task, err := idx.AddDocuments(docs, "id")
		if err != nil {
			return indexed, fmt.Errorf("meilisearch bulk index failed: %w", err)
		}
		if err := d.waitTask(task); err != nil {
			return indexed, err
		}
		indexed += len(docs)
	}
	return indexed, nil
}

// 从 Meilisearch 删除一个商品
func (d *MeiliProductDB) DeleteProduct(id uint) error {
	if err := d.ensureReady(); err != nil {
		return err
	}
	if id == 0 {
		return fmt.Errorf("invalid product id")
	}

	task, err := d.client.Index(d.index).DeleteDocument(strconv.FormatUint(uint64(id), 10))
	if err != nil {
		return fmt.Errorf("meilisearch delete product failed: %w", err)
	}
	return d.waitTask(task)
}

// 懒初始化：确认索引存在 + 配置可过滤/可排序/可搜索字段
// 成功后打 ready 标记，后续调用直接返回；失败不缓存，下次调用会重试
// （不缓存错误是因为 Meilisearch 可能比本进程晚启动，缓存了会一直搜不了）
func (d *MeiliProductDB) ensureReady() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.ready {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 1. 索引不存在就创建（主键用商品 id）
	if err := d.ensureIndex(ctx); err != nil {
		return err
	}

	idx := d.client.Index(d.index)
	// 2. 可过滤字段（status / category_id 参与过滤）
	if task, err := idx.UpdateFilterableAttributes(&[]string{"status", "category_id"}); err != nil {
		return fmt.Errorf("meilisearch set filterable attributes failed: %w", err)
	} else if err := d.waitTaskWithContext(ctx, task); err != nil {
		return err
	}

	// 3. 可排序字段（created_at 参与排序）
	if task, err := idx.UpdateSortableAttributes(&[]string{"created_at"}); err != nil {
		return fmt.Errorf("meilisearch set sortable attributes failed: %w", err)
	} else if err := d.waitTaskWithContext(ctx, task); err != nil {
		return err
	}

	// 4. 可搜索字段：name 在前优先级高（相当于 ES 的 name^3）
	if task, err := idx.UpdateSearchableAttributes(&[]string{"name", "description"}); err != nil {
		return fmt.Errorf("meilisearch set searchable attributes failed: %w", err)
	} else if err := d.waitTaskWithContext(ctx, task); err != nil {
		return err
	}

	d.ready = true
	return nil
}

// 索引不存在就创建
// web 和 worker 是两个进程、各自持有一个 MeiliProductDB，首次启动可能同时建索引，
// 所以 CreateIndex 报 already exists 时不能当成失败
func (d *MeiliProductDB) ensureIndex(ctx context.Context) error {
	if _, err := d.client.GetIndex(d.index); err == nil {
		return nil // 索引已存在，不用建
	}

	task, err := d.client.CreateIndex(&meilisearch.IndexConfig{Uid: d.index, PrimaryKey: "id"})
	if err != nil {
		// 可能是另一个进程抢先建好了，再查一次确认，确认到了就当成功
		if _, getErr := d.client.GetIndex(d.index); getErr == nil {
			return nil
		}
		return fmt.Errorf("meilisearch create index failed: %w", err)
	}
	return d.waitTaskWithContext(ctx, task)
}

// 等待异步任务完成，失败返回错误
// （相当于 ES 的 refresh=true：保证写入后立刻能搜到，不会出现"写了但搜不到"）
func (d *MeiliProductDB) waitTask(task *meilisearch.TaskInfo) error {
	return d.waitTaskWithContext(context.Background(), task)
}

func (d *MeiliProductDB) waitTaskWithContext(ctx context.Context, task *meilisearch.TaskInfo) error {
	if task == nil {
		return nil
	}
	t, err := d.client.WaitForTaskWithContext(ctx, task.TaskUID, 100*time.Millisecond)
	if err != nil {
		return fmt.Errorf("meilisearch wait task failed: %w", err)
	}
	if t.Status != meilisearch.TaskStatusSucceeded {
		return fmt.Errorf("meilisearch task %d finished with status %s", task.TaskUID, t.Status)
	}
	return nil
}

// meiliProductDoc 存入 Meilisearch 的文档结构（字段和原 ES 文档保持一致）
type meiliProductDoc struct {
	ID          uint                 `json:"id"`
	Name        string               `json:"name"`
	PriceCents  int64                `json:"price_cents"`
	Description string               `json:"description"`
	Stock       int                  `json:"stock"`
	CategoryID  uint                 `json:"category_id"`
	Status      string               `json:"status"`
	SaleCount   int                  `json:"sale_count"`
	Images      []meiliProductImage  `json:"images"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
}

type meiliProductImage struct {
	ID        uint   `json:"id"`
	ProductID uint   `json:"product_id"`
	URL       string `json:"url"`
	SortOrder int    `json:"sort_order"`
}

// mysql商品 -> Meilisearch 能识别的商品结构
func newMeiliProductDoc(product *model.Product) *meiliProductDoc {
	images := make([]meiliProductImage, 0, len(product.Images))
	for _, image := range product.Images {
		images = append(images, meiliProductImage{
			ID:        image.ID,
			ProductID: image.ProductID,
			URL:       image.URL,
			SortOrder: image.SortOrder,
		})
	}

	return &meiliProductDoc{
		ID:          product.ID,
		Name:        product.Name,
		PriceCents:  product.PriceCents,
		Description: product.Description,
		Stock:       product.Stock,
		CategoryID:  product.CategoryID,
		Status:      product.Status,
		SaleCount:   product.SaleCount,
		Images:      images,
		CreatedAt:   product.CreatedAt,
		UpdatedAt:   product.UpdatedAt,
	}
}

func (d meiliProductDoc) toModel() model.Product {
	images := make([]model.ProductImage, 0, len(d.Images))
	for _, image := range d.Images {
		images = append(images, model.ProductImage{
			ProductID: image.ProductID,
			URL:       image.URL,
			SortOrder: image.SortOrder,
		})
		images[len(images)-1].ID = image.ID
	}

	product := model.Product{
		Name:        d.Name,
		PriceCents:  d.PriceCents,
		Description: d.Description,
		Stock:       d.Stock,
		CategoryID:  d.CategoryID,
		Status:      d.Status,
		SaleCount:   d.SaleCount,
		Images:      images,
	}
	product.ID = d.ID
	product.CreatedAt = d.CreatedAt
	product.UpdatedAt = d.UpdatedAt

	return product
}
