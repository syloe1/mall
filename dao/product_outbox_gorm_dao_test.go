package dao

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mall/model"
)

// ─── helpers ────────────────────────────────────────────────────────────────────

func initTestOutboxDB(t *testing.T) *GormProductOutboxDB {
	t.Helper()
	db := newTestDB(t, &model.ProductOutbox{})
	return &GormProductOutboxDB{DB: db}
}

func seedOutbox(t *testing.T, db *GormProductOutboxDB, opts ...func(*model.ProductOutbox)) *model.ProductOutbox {
	t.Helper()
	e := &model.ProductOutbox{
		EventID:   "evt-" + randomSuffix(),
		EventType: model.ProductEventUpdated,
		ProductID: 1,
		Status:    model.ProductOutboxPending,
	}
	for _, opt := range opts {
		opt(e)
	}
	require.NoError(t, db.Create(e))
	return e
}

// ─── Create ─────────────────────────────────────────────────────────────────────

func TestGormOutboxDB_Create(t *testing.T) {
	g := initTestOutboxDB(t)
	e := seedOutbox(t, g)
	assert.NotZero(t, e.ID)
}

// ─── ListPending ────────────────────────────────────────────────────────────────

func TestGormOutboxDB_ListPending(t *testing.T) {
	g := initTestOutboxDB(t)
	seedOutbox(t, g)
	seedOutbox(t, g)
	// create one that's already published — should not appear
	seedOutbox(t, g, func(e *model.ProductOutbox) { e.Status = model.ProductOutboxPublished })

	list, err := g.ListPending(50)
	assert.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestGormOutboxDB_ListPending_DefaultLimit(t *testing.T) {
	g := initTestOutboxDB(t)
	// ListPending defaults limit to 50 when 0 is passed
	list, err := g.ListPending(0)
	assert.NoError(t, err)
	assert.NotNil(t, list)
}

// ─── MarkPublished ──────────────────────────────────────────────────────────────

func TestGormOutboxDB_MarkPublished(t *testing.T) {
	g := initTestOutboxDB(t)
	e := seedOutbox(t, g, func(e *model.ProductOutbox) { e.LastError = "prev-error" })

	assert.NoError(t, g.MarkPublished(e.ID))

	// Refetch to verify
	list, _ := g.ListPending(50)
	for _, item := range list {
		assert.NotEqual(t, e.ID, item.ID)
	}
}

// ─── MarkFailed ─────────────────────────────────────────────────────────────────

func TestGormOutboxDB_MarkFailed(t *testing.T) {
	g := initTestOutboxDB(t)
	e := seedOutbox(t, g)

	assert.NoError(t, g.MarkFailed(e.ID, "connection refused"))

	// Refetch to verify (it's no longer pending)
	list, _ := g.ListPending(50)
	for _, item := range list {
		assert.NotEqual(t, e.ID, item.ID)
	}
}

func TestGormOutboxDB_MarkFailed_RetryCountIncrements(t *testing.T) {
	g := initTestOutboxDB(t)
	e := seedOutbox(t, g)

	require.NoError(t, g.MarkFailed(e.ID, "error-1"))
	require.NoError(t, g.MarkFailed(e.ID, "error-2"))
	require.NoError(t, g.MarkFailed(e.ID, "error-3"))

	// The outbox entry is now in "failed" status, not "pending"
	// Verify via raw DB query (since ListPending won't return it)
	var updated model.ProductOutbox
	err := g.DB.First(&updated, e.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, model.ProductOutboxFailed, updated.Status)
	assert.Equal(t, 3, updated.RetryCount)
	assert.Equal(t, "error-3", updated.LastError)
}
