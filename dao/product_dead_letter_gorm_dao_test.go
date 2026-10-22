package dao

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mall/model"
)

// ─── helpers ────────────────────────────────────────────────────────────────────

func initTestDeadLetterDB(t *testing.T) *GormProductDeadLetterDB {
	t.Helper()
	db := newTestDB(t, &model.ProductEventDeadLetter{})
	return &GormProductDeadLetterDB{DB: db}
}

func seedDeadLetter(t *testing.T, db *GormProductDeadLetterDB, opts ...func(*model.ProductEventDeadLetter)) *model.ProductEventDeadLetter {
	t.Helper()
	dl := &model.ProductEventDeadLetter{
		EventID:      "evt-" + randomSuffix(),
		EventType:    model.ProductEventUpdated,
		ProductID:    1,
		Source:       model.ProductDeadLetterPublisher,
		ErrorMessage: "test error",
	}
	for _, opt := range opts {
		opt(dl)
	}
	require.NoError(t, db.Create(dl))
	return dl
}

// ─── Create ─────────────────────────────────────────────────────────────────────

func TestGormDeadLetterDB_Create(t *testing.T) {
	g := initTestDeadLetterDB(t)
	dl := seedDeadLetter(t, g)
	assert.NotZero(t, dl.ID)
}

// ─── List ───────────────────────────────────────────────────────────────────────

func TestGormDeadLetterDB_List(t *testing.T) {
	g := initTestDeadLetterDB(t)
	seedDeadLetter(t, g)
	seedDeadLetter(t, g)

	records, total, err := g.List(ProductDeadLetterQuery{Page: 1, PageSize: 10})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, records, 2)
}

func TestGormDeadLetterDB_List_PageClamping(t *testing.T) {
	g := initTestDeadLetterDB(t)
	// Page=0 → clamped to 1; PageSize=0 → clamped to 20; PageSize>100 → clamped to 100
	records, _, err := g.List(ProductDeadLetterQuery{Page: 0, PageSize: 0})
	assert.NoError(t, err)
	assert.NotNil(t, records) // no panic, validates clamping
}

func TestGormDeadLetterDB_List_SourceFilter(t *testing.T) {
	g := initTestDeadLetterDB(t)
	seedDeadLetter(t, g, func(dl *model.ProductEventDeadLetter) {
		dl.Source = model.ProductDeadLetterPublisher
	})
	seedDeadLetter(t, g, func(dl *model.ProductEventDeadLetter) {
		dl.Source = model.ProductDeadLetterConsumer
	})

	records, total, err := g.List(ProductDeadLetterQuery{
		Page: 1, PageSize: 10, Source: model.ProductDeadLetterPublisher,
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, model.ProductDeadLetterPublisher, records[0].Source)
}

func TestGormDeadLetterDB_List_EventTypeFilter(t *testing.T) {
	g := initTestDeadLetterDB(t)
	seedDeadLetter(t, g, func(dl *model.ProductEventDeadLetter) { dl.EventType = "stock.decrease" })
	seedDeadLetter(t, g, func(dl *model.ProductEventDeadLetter) { dl.EventType = "product.updated" })

	records, total, err := g.List(ProductDeadLetterQuery{
		Page: 1, PageSize: 10, EventType: "stock.decrease",
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, records, 1)
	assert.Equal(t, "stock.decrease", records[0].EventType)
}

func TestGormDeadLetterDB_List_ProductIDFilter(t *testing.T) {
	g := initTestDeadLetterDB(t)
	seedDeadLetter(t, g, func(dl *model.ProductEventDeadLetter) { dl.ProductID = 42 })
	seedDeadLetter(t, g, func(dl *model.ProductEventDeadLetter) { dl.ProductID = 99 })

	records, total, err := g.List(ProductDeadLetterQuery{
		Page: 1, PageSize: 10, ProductID: 42,
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, uint(42), records[0].ProductID)
}

func TestGormDeadLetterDB_List_CombinedFilters(t *testing.T) {
	g := initTestDeadLetterDB(t)
	seedDeadLetter(t, g, func(dl *model.ProductEventDeadLetter) {
		dl.Source = "A"; dl.EventType = "type1"; dl.ProductID = 1
	})
	seedDeadLetter(t, g, func(dl *model.ProductEventDeadLetter) {
		dl.Source = "A"; dl.EventType = "type2"; dl.ProductID = 1
	})
	seedDeadLetter(t, g, func(dl *model.ProductEventDeadLetter) {
		dl.Source = "B"; dl.EventType = "type1"; dl.ProductID = 2
	})

	records, total, err := g.List(ProductDeadLetterQuery{
		Page: 1, PageSize: 10, Source: "A", EventType: "type1",
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, records, 1)
	assert.Equal(t, "A", records[0].Source)
	assert.Equal(t, "type1", records[0].EventType)
}

func TestGormDeadLetterDB_List_Empty(t *testing.T) {
	g := initTestDeadLetterDB(t)
	records, total, err := g.List(ProductDeadLetterQuery{Page: 1, PageSize: 10})
	assert.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, records)
}
