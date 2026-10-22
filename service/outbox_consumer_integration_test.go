package service

import (
	"context"
	"errors"
	"fmt"
	"mall/dao"
	"mall/database"
	"mall/model"
	"mall/pkg/mq"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestProductTransactionRollbackDoesNotCreateOutbox(t *testing.T) {
	db := newTestDB(t)
	productDB := &dao.GormProductDB{DB: db}

	err := productDB.Transaction(func(txProductDB dao.ProductDB, txOutboxDB dao.ProductOutboxDB) error {
		product := &model.Product{
			Name:       "rollback-product",
			Status:     model.ProductOnSale,
			Stock:      10,
			PriceCents: 1000,
		}
		if err := txProductDB.Create(product); err != nil {
			return err
		}
		if err := txOutboxDB.Create(&model.ProductOutbox{
			EventID:   "rollback-event",
			EventType: model.ProductEventCreated,
			ProductID: product.ID,
			Status:    model.ProductOutboxPending,
		}); err != nil {
			return err
		}
		return errors.New("force rollback")
	})
	if err == nil {
		t.Fatal("expected transaction rollback error")
	}

	var outboxCount int64
	if err := db.Model(&model.ProductOutbox{}).Count(&outboxCount).Error; err != nil {
		t.Fatalf("count product outbox failed: %v", err)
	}
	if outboxCount != 0 {
		t.Fatalf("expected no outbox after rollback, got %d", outboxCount)
	}
}

func TestProductOutboxPublisherRabbitMQFailureCreatesDeadLetter(t *testing.T) {
	db := newTestDB(t)
	outboxDB := &dao.GormProductOutboxDB{DB: db}
	deadLetterDB := &dao.GormProductDeadLetterDB{DB: db}
	producer := &failingProductProducer{err: errors.New("rabbitmq down")}

	outbox := &model.ProductOutbox{
		EventID:   "event-publisher-failed",
		EventType: model.ProductEventCreated,
		ProductID: 1,
		Status:    model.ProductOutboxPending,
	}
	if err := outboxDB.Create(outbox); err != nil {
		t.Fatalf("create outbox failed: %v", err)
	}

	publisher := NewProductOutboxPublisher(outboxDB, deadLetterDB, producer)
	publisher.publishOnce(context.Background())

	if producer.calls != 3 {
		t.Fatalf("expected 3 publish attempts, got %d", producer.calls)
	}

	var updated model.ProductOutbox
	if err := db.First(&updated, outbox.ID).Error; err != nil {
		t.Fatalf("get outbox failed: %v", err)
	}
	if updated.Status != model.ProductOutboxFailed {
		t.Fatalf("expected outbox failed status, got %s", updated.Status)
	}

	var deadLetters []model.ProductEventDeadLetter
	if err := db.Find(&deadLetters).Error; err != nil {
		t.Fatalf("list dead letters failed: %v", err)
	}
	if len(deadLetters) != 1 {
		t.Fatalf("expected 1 dead letter, got %d", len(deadLetters))
	}
	if deadLetters[0].Source != model.ProductDeadLetterPublisher {
		t.Fatalf("expected publisher dead letter, got %s", deadLetters[0].Source)
	}
}

func TestProductSearchConsumerFailureRetriesThenCreatesDeadLetter(t *testing.T) {
	db := newTestDB(t)
	productDB := &dao.GormProductDB{DB: db}
	deadLetterDB := &dao.GormProductDeadLetterDB{DB: db}
	searchDB := &failingProductSearchDB{err: errors.New("meilisearch down")}

	product := &model.Product{
		Name:       "search-failed-product",
		Status:     model.ProductOnSale,
		Stock:      10,
		PriceCents: 1000,
	}
	if err := productDB.Create(product); err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	msg := mq.Message{
		MsgID:     "event-consumer-failed",
		Event:     model.ProductEventUpdated,
		BizID:     product.ID,
		CreatedAt: time.Now(),
	}

	consumer := &ProductSearchConsumer{
		productDB:    productDB,
		searchDB:     searchDB,
		deadLetterDB: deadLetterDB,
	}
	// handle 内部重试 3 次，仍失败则写死信表并返回 nil（确认消息，避免无限重入队）
	if err := consumer.handle(context.Background(), msg); err != nil {
		t.Fatalf("handle should not return error after dead-lettering: %v", err)
	}

	if searchDB.indexCalls != 3 {
		t.Fatalf("expected 3 search index attempts, got %d", searchDB.indexCalls)
	}

	var deadLetters []model.ProductEventDeadLetter
	if err := db.Find(&deadLetters).Error; err != nil {
		t.Fatalf("list dead letters failed: %v", err)
	}
	if len(deadLetters) != 1 {
		t.Fatalf("expected 1 dead letter, got %d", len(deadLetters))
	}
	if deadLetters[0].Source != model.ProductDeadLetterConsumer {
		t.Fatalf("expected consumer dead letter, got %s", deadLetters[0].Source)
	}
}

type failingProductProducer struct {
	err   error
	calls int
}

func (p *failingProductProducer) PublishProductEvent(ctx context.Context, event mq.ProductEvent) error {
	p.calls++
	return p.err
}

type failingProductSearchDB struct {
	err         error
	indexCalls  int
	deleteCalls int
}

func (d *failingProductSearchDB) SearchPublic(query dao.ProductListQuery) ([]model.Product, int64, error) {
	return nil, 0, d.err
}

func (d *failingProductSearchDB) IndexProduct(product *model.Product) error {
	d.indexCalls++
	return d.err
}

func (d *failingProductSearchDB) ReplaceAllProducts(products []model.Product) (int, error) {
	d.indexCalls++
	return 0, d.err
}

func (d *failingProductSearchDB) DeleteProduct(id uint) error {
	d.deleteCalls++
	return d.err
}

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	return db
}
