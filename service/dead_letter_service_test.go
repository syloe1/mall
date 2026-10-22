package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"mall/dao"
	"mall/model"
)

// ─── local mock ──────────────────────────────────────────────────────────────

// mockProductDeadLetterDB implements dao.ProductDeadLetterDB for testing.
type mockProductDeadLetterDB struct {
	// configured return values
	records []model.ProductEventDeadLetter
	total   int64
	err     error

	// captured last List call
	lastQuery dao.ProductDeadLetterQuery
}

func (m *mockProductDeadLetterDB) Create(record *model.ProductEventDeadLetter) error {
	// placeholder — not called by DeadLetterService yet
	return nil
}

func (m *mockProductDeadLetterDB) List(query dao.ProductDeadLetterQuery) ([]model.ProductEventDeadLetter, int64, error) {
	m.lastQuery = query
	return m.records, m.total, m.err
}

// ─── helper ───────────────────────────────────────────────────────────────────

func newTestDeadLetterService() (*DeadLetterService, *mockProductDeadLetterDB) {
	db := &mockProductDeadLetterDB{}
	return NewDeadLetterService(db), db
}

// ─── tests ────────────────────────────────────────────────────────────────────

func TestDeadLetterService_ListProductDeadLetters(t *testing.T) {

	t.Run("defaults_page_and_page_size", func(t *testing.T) {
		svc, db := newTestDeadLetterService()
		db.records = []model.ProductEventDeadLetter{}
		db.total = 0

		result, err := svc.ListProductDeadLetters(ListProductDeadLettersInput{
			Page:     0,
			PageSize: 0,
		})

		assert.NoError(t, err)
		assert.NotNil(t, result)

		// 断言传给 DAO 的 query 使用了默认值
		assert.Equal(t, 1, db.lastQuery.Page, "DAO Page should default to 1")
		assert.Equal(t, 20, db.lastQuery.PageSize, "DAO PageSize should default to 20")

		// 断言 result 中也反映了默认值
		assert.Equal(t, 1, result.Page)
		assert.Equal(t, 20, result.PageSize)
	})

	t.Run("caps_page_size", func(t *testing.T) {
		svc, db := newTestDeadLetterService()
		db.records = []model.ProductEventDeadLetter{}

		result, err := svc.ListProductDeadLetters(ListProductDeadLettersInput{
			Page:     1,
			PageSize: 200,
		})

		assert.NoError(t, err)
		assert.NotNil(t, result)

		// 断言 DAO query 的 PageSize 被限制为 100
		assert.Equal(t, 100, db.lastQuery.PageSize, "DAO PageSize should be capped at 100")

		// 断言 result 也使用 capped 后的值
		assert.Equal(t, 100, result.PageSize)
	})

	t.Run("passes_filters_and_returns_records", func(t *testing.T) {
		svc, db := newTestDeadLetterService()
		wantRecords := []model.ProductEventDeadLetter{
			{EventID: "evt-1", EventType: "product_updated", ProductID: 10, Source: "publisher"},
			{EventID: "evt-2", EventType: "product_updated", ProductID: 10, Source: "publisher"},
		}
		db.records = wantRecords
		db.total = 42

		result, err := svc.ListProductDeadLetters(ListProductDeadLettersInput{
			Page:      2,
			PageSize:  30,
			Source:    "publisher",
			EventType: "product_updated",
			ProductID: 10,
		})

		assert.NoError(t, err)
		assert.NotNil(t, result)

		// 断言筛选字段完整透传到 DAO
		assert.Equal(t, 2, db.lastQuery.Page)
		assert.Equal(t, 30, db.lastQuery.PageSize)
		assert.Equal(t, "publisher", db.lastQuery.Source)
		assert.Equal(t, "product_updated", db.lastQuery.EventType)
		assert.Equal(t, uint(10), db.lastQuery.ProductID)

		// 断言返回结果正确
		assert.Equal(t, wantRecords, result.Items)
		assert.Equal(t, int64(42), result.Total)
		assert.Equal(t, 2, result.Page)
		assert.Equal(t, 30, result.PageSize)
	})

	t.Run("returns_db_error", func(t *testing.T) {
		svc, db := newTestDeadLetterService()
		dbErr := errors.New("database connection lost")
		db.err = dbErr

		result, err := svc.ListProductDeadLetters(ListProductDeadLettersInput{
			Page:     1,
			PageSize: 10,
		})

		assert.Error(t, err)
		assert.Equal(t, dbErr, err)
		assert.Nil(t, result, "result should be nil when DAO returns error")
	})

	t.Run("negative_page_and_page_size_use_defaults", func(t *testing.T) {
		svc, db := newTestDeadLetterService()
		db.records = []model.ProductEventDeadLetter{}

		result, err := svc.ListProductDeadLetters(ListProductDeadLettersInput{
			Page:     -5,
			PageSize: -10,
		})

		assert.NoError(t, err)
		assert.NotNil(t, result)

		// 断言负数值时使用默认值
		assert.Equal(t, 1, db.lastQuery.Page, "negative Page should default to 1")
		assert.Equal(t, 20, db.lastQuery.PageSize, "negative PageSize should default to 20")
		assert.Equal(t, 1, result.Page)
		assert.Equal(t, 20, result.PageSize)
	})
}
