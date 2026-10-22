package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	mt "github.com/stretchr/testify/mock"
	"gorm.io/gorm"

	"mall/mock"
	"mall/model"
	"mall/pkg/mq"
)

func initPublisherTest(outboxDB *mock.MockProductOutboxDB, deadLetterDB *mock.MockProductDeadLetterDB, producer *mock.MockProductEventProducer) *ProductOutboxPublisher {
	return &ProductOutboxPublisher{
		outboxDB:     outboxDB,
		deadLetterDB: deadLetterDB,
		producer:     producer,
		interval:     2 * time.Second,
		limit:        50,
	}
}

// ─── NewProductOutboxPublisher ────────────────────────────────────────────────

func TestNewProductOutboxPublisher(t *testing.T) {
	outboxDB := new(mock.MockProductOutboxDB)
	deadLetterDB := new(mock.MockProductDeadLetterDB)
	producer := new(mock.MockProductEventProducer)

	p := NewProductOutboxPublisher(outboxDB, deadLetterDB, producer)

	assert.NotNil(t, p)
	assert.Equal(t, outboxDB, p.outboxDB)
	assert.Equal(t, deadLetterDB, p.deadLetterDB)
	assert.Equal(t, producer, p.producer)
	assert.Equal(t, 2*time.Second, p.interval)
	assert.Equal(t, 50, p.limit)
}

// ─── Start nil guard ──────────────────────────────────────────────────────────

func TestStart_NilPublisher(t *testing.T) {
	var p *ProductOutboxPublisher
	ctx := context.Background()
	// nil receiver → should return immediately without panic
	p.Start(ctx)
}

func TestStart_NilOutboxDB(t *testing.T) {
	p := &ProductOutboxPublisher{}
	ctx := context.Background()
	p.Start(ctx)
}

func TestStart_NilProducer(t *testing.T) {
	outboxDB := new(mock.MockProductOutboxDB)
	p := &ProductOutboxPublisher{outboxDB: outboxDB}
	ctx := context.Background()
	p.Start(ctx)
}

// ─── publishOnce: empty outbox ────────────────────────────────────────────────

func TestPublishOnce_EmptyOutbox(t *testing.T) {
	outboxDB := new(mock.MockProductOutboxDB)
	deadLetterDB := new(mock.MockProductDeadLetterDB)
	producer := new(mock.MockProductEventProducer)
	p := initPublisherTest(outboxDB, deadLetterDB, producer)

	outboxDB.On("ListPending", 50).Return([]model.ProductOutbox{}, nil).Once()

	p.publishOnce(context.Background())

	// producer 不应被调用
	producer.AssertNotCalled(t, "PublishProductEvent")
	outboxDB.AssertNotCalled(t, "MarkPublished")
	outboxDB.AssertNotCalled(t, "MarkFailed")
	outboxDB.AssertExpectations(t)
}

// ─── publishOnce: list pending error ──────────────────────────────────────────

func TestPublishOnce_ListPendingError(t *testing.T) {
	outboxDB := new(mock.MockProductOutboxDB)
	deadLetterDB := new(mock.MockProductDeadLetterDB)
	producer := new(mock.MockProductEventProducer)
	p := initPublisherTest(outboxDB, deadLetterDB, producer)

	outboxDB.On("ListPending", 50).Return(
		[]model.ProductOutbox(nil), errors.New("db connection lost"),
	).Once()

	p.publishOnce(context.Background())

	producer.AssertNotCalled(t, "PublishProductEvent")
	outboxDB.AssertExpectations(t)
}

// ─── publishOnce: single event, publish success ───────────────────────────────

func TestPublishOnce_PublishSuccess(t *testing.T) {
	outboxDB := new(mock.MockProductOutboxDB)
	deadLetterDB := new(mock.MockProductDeadLetterDB)
	producer := new(mock.MockProductEventProducer)
	p := initPublisherTest(outboxDB, deadLetterDB, producer)

	event := model.ProductOutbox{
		Model:     gorm.Model{ID: 1},
		EventID:   "evt-001",
		EventType: model.ProductEventCreated,
		ProductID: 100,
	}
	outboxDB.On("ListPending", 50).Return([]model.ProductOutbox{event}, nil).Once()
	producer.On("PublishProductEvent", mt.Anything, mt.MatchedBy(func(e mq.ProductEvent) bool {
		return e.EventID == "evt-001" && e.ProductID == 100 && e.EventType == model.ProductEventCreated
	})).Return(nil).Once()
	outboxDB.On("MarkPublished", uint(1)).Return(nil).Once()

	p.publishOnce(context.Background())

	outboxDB.AssertExpectations(t)
	producer.AssertExpectations(t)
	outboxDB.AssertNotCalled(t, "MarkFailed")
}

// ─── publishOnce: single event, publish fails → mark failed + dead letter ─────

func TestPublishOnce_PublishFailed(t *testing.T) {
	outboxDB := new(mock.MockProductOutboxDB)
	deadLetterDB := new(mock.MockProductDeadLetterDB)
	producer := new(mock.MockProductEventProducer)
	p := initPublisherTest(outboxDB, deadLetterDB, producer)

	event := model.ProductOutbox{
		Model:     gorm.Model{ID: 2},
		EventID:   "evt-002",
		EventType: model.ProductEventDeleted,
		ProductID: 200,
	}
	publishErr := errors.New("rabbitmq connection refused")

	outboxDB.On("ListPending", 50).Return([]model.ProductOutbox{event}, nil).Once()
	// retryx 重试 3 次，每次都失败
	producer.On("PublishProductEvent", mt.Anything, mt.AnythingOfType("mq.ProductEvent")).
		Return(publishErr)
	outboxDB.On("MarkFailed", uint(2), publishErr.Error()).Return(nil).Once()
	deadLetterDB.On("Create", mt.MatchedBy(func(dl *model.ProductEventDeadLetter) bool {
		return dl.EventID == "evt-002" &&
			dl.ProductID == 200 &&
			dl.Source == model.ProductDeadLetterPublisher &&
			dl.RetryCount == 1
	})).Return(nil).Once()

	p.publishOnce(context.Background())

	outboxDB.AssertExpectations(t)
	producer.AssertExpectations(t)
	deadLetterDB.AssertExpectations(t)
	outboxDB.AssertNotCalled(t, "MarkPublished")

	// 确认 producer 被重试了 3 次
	producer.AssertNumberOfCalls(t, "PublishProductEvent", 3)
}

// ─── publishOnce: mark published error ────────────────────────────────────────

func TestPublishOnce_MarkPublishedError(t *testing.T) {
	outboxDB := new(mock.MockProductOutboxDB)
	deadLetterDB := new(mock.MockProductDeadLetterDB)
	producer := new(mock.MockProductEventProducer)
	p := initPublisherTest(outboxDB, deadLetterDB, producer)

	event := model.ProductOutbox{
		Model:     gorm.Model{ID: 3},
		EventID:   "evt-003",
		EventType: model.ProductEventUpdated,
		ProductID: 300,
	}
	outboxDB.On("ListPending", 50).Return([]model.ProductOutbox{event}, nil).Once()
	producer.On("PublishProductEvent", mt.Anything, mt.AnythingOfType("mq.ProductEvent")).
		Return(nil).Once()
	outboxDB.On("MarkPublished", uint(3)).Return(errors.New("db error")).Once()

	p.publishOnce(context.Background())

	outboxDB.AssertExpectations(t)
	producer.AssertExpectations(t)
	outboxDB.AssertNotCalled(t, "MarkFailed")
	deadLetterDB.AssertNotCalled(t, "Create")
}

// ─── publishOnce: failure but deadLetterDB is nil ─────────────────────────────

func TestPublishOnce_DeadLetterDBNil(t *testing.T) {
	outboxDB := new(mock.MockProductOutboxDB)
	producer := new(mock.MockProductEventProducer)
	// deadLetterDB 为 nil
	p := &ProductOutboxPublisher{
		outboxDB: outboxDB,
		producer: producer,
		limit:    50,
	}

	event := model.ProductOutbox{
		Model:     gorm.Model{ID: 4},
		EventID:   "evt-004",
		EventType: model.ProductEventCreated,
		ProductID: 400,
	}
	publishErr := errors.New("rabbitmq down")

	outboxDB.On("ListPending", 50).Return([]model.ProductOutbox{event}, nil).Once()
	producer.On("PublishProductEvent", mt.Anything, mt.AnythingOfType("mq.ProductEvent")).
		Return(publishErr)
	outboxDB.On("MarkFailed", uint(4), publishErr.Error()).Return(nil).Once()

	p.publishOnce(context.Background())

	outboxDB.AssertExpectations(t)
	producer.AssertExpectations(t)
}

// ─── recordDeadLetter: nil deadLetterDB ───────────────────────────────────────

func TestRecordDeadLetter_NilDB(t *testing.T) {
	outboxDB := new(mock.MockProductOutboxDB)
	producer := new(mock.MockProductEventProducer)
	p := &ProductOutboxPublisher{
		outboxDB:     outboxDB,
		deadLetterDB: nil,
		producer:     producer,
	}

	p.recordDeadLetter(
		context.Background(),
		mq.ProductEvent{EventID: "e", EventType: "t", ProductID: 1},
		model.ProductDeadLetterPublisher,
		errors.New("some error"),
		2,
	)
	// 不 panic 即通过
}

// ─── recordDeadLetter: success ────────────────────────────────────────────────

func TestRecordDeadLetter_Success(t *testing.T) {
	outboxDB := new(mock.MockProductOutboxDB)
	deadLetterDB := new(mock.MockProductDeadLetterDB)
	producer := new(mock.MockProductEventProducer)
	p := initPublisherTest(outboxDB, deadLetterDB, producer)

	testErr := errors.New("network timeout")
	deadLetterDB.On("Create", mt.MatchedBy(func(dl *model.ProductEventDeadLetter) bool {
		return dl.EventID == "evt-005" &&
			dl.EventType == "product.updated" &&
			dl.ProductID == 500 &&
			dl.Source == model.ProductDeadLetterPublisher &&
			dl.RetryCount == 3 &&
			dl.ErrorMessage == testErr.Error() &&
			dl.Payload != "" // payload 不应为空
	})).Return(nil).Once()

	p.recordDeadLetter(
		context.Background(),
		mq.ProductEvent{EventID: "evt-005", EventType: "product.updated", ProductID: 500},
		model.ProductDeadLetterPublisher,
		testErr,
		3,
	)

	deadLetterDB.AssertExpectations(t)
}

// ─── publishOnce: multiple events mixed results ───────────────────────────────

func TestPublishOnce_MultipleEvents(t *testing.T) {
	outboxDB := new(mock.MockProductOutboxDB)
	deadLetterDB := new(mock.MockProductDeadLetterDB)
	producer := new(mock.MockProductEventProducer)
	p := initPublisherTest(outboxDB, deadLetterDB, producer)

	events := []model.ProductOutbox{
		{Model: gorm.Model{ID: 1}, EventID: "ok-1", EventType: "created", ProductID: 1},
		{Model: gorm.Model{ID: 2}, EventID: "fail-1", EventType: "deleted", ProductID: 2},
		{Model: gorm.Model{ID: 3}, EventID: "ok-2", EventType: "updated", ProductID: 3},
	}
	failErr := errors.New("rabbitmq error")

	outboxDB.On("ListPending", 50).Return(events, nil).Once()

	// 成功事件 1
	producer.On("PublishProductEvent", mt.Anything, mt.MatchedBy(func(e mq.ProductEvent) bool {
		return e.EventID == "ok-1"
	})).Return(nil).Once()
	outboxDB.On("MarkPublished", uint(1)).Return(nil).Once()

	// 失败事件
	producer.On("PublishProductEvent", mt.Anything, mt.MatchedBy(func(e mq.ProductEvent) bool {
		return e.EventID == "fail-1"
	})).Return(failErr)
	outboxDB.On("MarkFailed", uint(2), failErr.Error()).Return(nil).Once()
	deadLetterDB.On("Create", mt.MatchedBy(func(dl *model.ProductEventDeadLetter) bool {
		return dl.EventID == "fail-1"
	})).Return(nil).Once()

	// 成功事件 2
	producer.On("PublishProductEvent", mt.Anything, mt.MatchedBy(func(e mq.ProductEvent) bool {
		return e.EventID == "ok-2"
	})).Return(nil).Once()
	outboxDB.On("MarkPublished", uint(3)).Return(nil).Once()

	p.publishOnce(context.Background())

	outboxDB.AssertExpectations(t)
	deadLetterDB.AssertExpectations(t)
}
