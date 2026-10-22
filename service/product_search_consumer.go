package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"mall/dao"
	"mall/model"
	"mall/pkg/mq"
	"mall/pkg/retryx"
	"time"

	"gorm.io/gorm"
)

// ProductSearchConsumer 商品索引同步消费者
// 监听 RabbitMQ 商品事件队列 → 同步商品数据到 Meilisearch（搜索用）
// （原来监听 Kafka → 同步到 ElasticSearch；传输层和搜索引擎都换了，职责不变）
type ProductSearchConsumer struct {
	consumer     *mq.Consumer           // RabbitMQ 消费者客户端
	productDB    dao.ProductDB          // 操作商品表（MySQL）
	searchDB     dao.ProductSearchDB    // 操作 Meilisearch（搜索引擎）
	deadLetterDB dao.ProductDeadLetterDB // 失败消息存死信队列（MySQL）
}

func NewProductSearchConsumer(consumer *mq.Consumer, productDB dao.ProductDB, searchDB dao.ProductSearchDB, deadLetterDB dao.ProductDeadLetterDB) *ProductSearchConsumer {
	return &ProductSearchConsumer{
		consumer:     consumer,
		productDB:    productDB,
		searchDB:     searchDB,
		deadLetterDB: deadLetterDB,
	}
}

// Start 启动消费者，监听商品队列（一次最多预取 10 条）
func (c *ProductSearchConsumer) Start(ctx context.Context) error {
	return c.consumer.Start(mq.ProductQueue, 10, c.handle)
}

func (c *ProductSearchConsumer) Close() error {
	if c == nil || c.consumer == nil {
		return nil
	}
	return c.consumer.Close()
}

// handle 处理一条商品事件消息
// 重试 3 次，还是失败 → 写死信表 → 返回 nil 确认消息（防止无限重入队）
func (c *ProductSearchConsumer) handle(ctx context.Context, msg mq.Message) error {
	err := retryx.Do(ctx, 3, 200*time.Millisecond, func() error {
		return c.handleEvent(msg)
	})
	if err != nil {
		// 处理失败 → 记录到死信表（人工后续处理）
		c.recordDeadLetter(msg, err)
		slog.Error("dead-letter product event", "msg_id", msg.MsgID, "event", msg.Event, "product_id", msg.BizID, "error", err)
	}
	return nil
}

// 处理单条商品事件：商品新增/更新/删除
func (c *ProductSearchConsumer) handleEvent(msg mq.Message) error {
	switch msg.Event {
	// 商品被删除 → 从 Meilisearch 里删除
	case model.ProductEventDeleted:
		return c.searchDB.DeleteProduct(msg.BizID)
		// 商品创建 / 更新 → 重新同步到 Meilisearch
	case model.ProductEventCreated, model.ProductEventUpdated:
		return c.syncProduct(msg.BizID)
	default:
		slog.Warn("ignore unknown product event", "msg_id", msg.MsgID, "event", msg.Event)
		return nil
	}
}

func (c *ProductSearchConsumer) syncProduct(productID uint) error {
	product, err := c.productDB.GetProductById(productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.searchDB.DeleteProduct(productID)
		}
		return err
	}
	// 因为搜索页面只展示“在售商品”，下架/删除的商品从索引里拿掉
	if product.Status != model.ProductOnSale {
		if err := c.searchDB.DeleteProduct(productID); err != nil {
			return err
		}
		slog.Info("removed product from search index", "product_id", productID)
		return nil
	}
	if err := c.searchDB.IndexProduct(product); err != nil {
		return err
	}
	slog.Info("indexed product", "product_id", productID, "name", product.Name)
	return nil
}

// 记录处理失败的商品事件 → 存入数据库死信表
func (c *ProductSearchConsumer) recordDeadLetter(msg mq.Message, err error) {
	if c.deadLetterDB == nil {
		return
	}
	payload, _ := json.Marshal(msg)
	// 创建一条死信记录，存入数据库
	_ = c.deadLetterDB.Create(&model.ProductEventDeadLetter{
		EventID:      msg.MsgID,                       // 事件唯一ID
		EventType:    msg.Event,                       // 事件类型：创建/更新/删除
		ProductID:    msg.BizID,                       // 商品ID
		Source:       model.ProductDeadLetterConsumer, // 来源：消费者失败
		RetryCount:   3,                               // 已重试 3 次
		ErrorMessage: err.Error(),                     // 失败原因（如：Meilisearch 连接失败）
		Payload:      string(payload),                 // 完整原始消息（最重要！）
	})
}
