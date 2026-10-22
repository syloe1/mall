package service

import (
	"errors"
	"mall/model"
	"mall/pkg/core"
	seckillpkg "mall/pkg/seckill"
	"net/http"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type mockSeckillDB struct {
	activity       *model.SeckillActivity
	listActivities []*model.SeckillActivity
	getErr         error
	createErr      error
	created        []*model.SeckillPurchase
	lastPurchase   *model.SeckillPurchase
	getCalls       int
	createCalls    int
}

func (m *mockSeckillDB) GetActivityByID(activityID uint) (*model.SeckillActivity, error) {
	m.getCalls++
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.activity == nil || m.activity.ID != activityID {
		return nil, gorm.ErrRecordNotFound
	}
	return m.activity, nil
}

func (m *mockSeckillDB) ListActiveActivities() ([]*model.SeckillActivity, error) {
	return m.listActivities, nil
}

func (m *mockSeckillDB) GetPurchase(userID, activityID uint) (*model.SeckillPurchase, error) {
	return nil, gorm.ErrRecordNotFound
}

func (m *mockSeckillDB) CreatePurchase(purchase *model.SeckillPurchase) error {
	m.createCalls++
	m.lastPurchase = purchase
	if m.createErr != nil {
		return m.createErr
	}
	m.created = append(m.created, purchase)
	return nil
}

func newSeckillTestService(t *testing.T, db *mockSeckillDB) (*SeckillService, *miniredis.Miniredis, *redis.Client) {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
		mr.Close()
	})

	return NewSeckillService(rdb, db, nil), mr, rdb
}

func runningActivity(id uint) *model.SeckillActivity {
	now := time.Now()
	activity := &model.SeckillActivity{
		ProductID:          1001,
		ProductName:        "seckill product",
		OriginalPriceCents: 10000,
		SeckillPriceCents:  1000,
		TotalStock:         10,
		AvailableStock:     3,
		PerLimit:           1,
		StartAt:            now.Add(-time.Minute),
		EndAt:              now.Add(time.Hour),
		Status:             model.SeckillActivityRunning,
	}
	activity.ID = id
	return activity
}

func assertRedisValue(t *testing.T, mr *miniredis.Miniredis, key string, want string) {
	t.Helper()
	got, err := mr.Get(key)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestSeckillServiceInitActivityStock(t *testing.T) {
	t.Run("invalid_activity_id", func(t *testing.T) {
		svc, _, _ := newSeckillTestService(t, &mockSeckillDB{})

		err := svc.InitActivityStock(0)

		assert.Error(t, err)
	})

	t.Run("activity_not_found", func(t *testing.T) {
		svc, _, _ := newSeckillTestService(t, &mockSeckillDB{getErr: gorm.ErrRecordNotFound})

		err := svc.InitActivityStock(1)

		assert.Error(t, err)
	})

	t.Run("closed_or_ended_activity", func(t *testing.T) {
		closed := runningActivity(1)
		closed.Status = model.SeckillActivityClosed
		svc, _, _ := newSeckillTestService(t, &mockSeckillDB{activity: closed})

		err := svc.InitActivityStock(1)

		assert.Error(t, err)

		ended := runningActivity(2)
		ended.Status = model.SeckillActivityEnded
		svc, _, _ = newSeckillTestService(t, &mockSeckillDB{activity: ended})

		err = svc.InitActivityStock(2)

		assert.Error(t, err)
	})

	t.Run("activity_has_ended", func(t *testing.T) {
		activity := runningActivity(1)
		activity.EndAt = time.Now().Add(-2 * time.Hour)
		svc, _, _ := newSeckillTestService(t, &mockSeckillDB{activity: activity})

		err := svc.InitActivityStock(1)

		assert.Error(t, err)
	})

	t.Run("success_and_duplicate_init", func(t *testing.T) {
		activity := runningActivity(1)
		svc, mr, _ := newSeckillTestService(t, &mockSeckillDB{activity: activity})

		err := svc.InitActivityStock(1)

		require.NoError(t, err)
		assertRedisValue(t, mr, seckillpkg.StockKey(1), "3")

		err = svc.InitActivityStock(1)

		assert.Error(t, err)
	})
}

func TestSeckillServiceBuy(t *testing.T) {
	t.Run("invalid_ids", func(t *testing.T) {
		svc, _, _ := newSeckillTestService(t, &mockSeckillDB{})

		assert.Error(t, svc.Buy(0, 1))
		assert.Error(t, svc.Buy(1, 0))
	})

	t.Run("activity_status_and_time_invalid", func(t *testing.T) {
		draft := runningActivity(1)
		draft.Status = model.SeckillActivityDraft
		svc, _, _ := newSeckillTestService(t, &mockSeckillDB{activity: draft})
		assert.Error(t, svc.Buy(1, 10))

		upcoming := runningActivity(2)
		upcoming.StartAt = time.Now().Add(time.Hour)
		upcoming.EndAt = time.Now().Add(2 * time.Hour)
		svc, _, _ = newSeckillTestService(t, &mockSeckillDB{activity: upcoming})
		assert.Error(t, svc.Buy(2, 10))

		ended := runningActivity(3)
		ended.EndAt = time.Now().Add(-time.Minute)
		svc, _, _ = newSeckillTestService(t, &mockSeckillDB{activity: ended})
		assert.Error(t, svc.Buy(3, 10))
	})

	t.Run("sold_out", func(t *testing.T) {
		activity := runningActivity(1)
		svc, mr, _ := newSeckillTestService(t, &mockSeckillDB{activity: activity})
		mr.Set(seckillpkg.StockKey(1), "0")

		err := svc.Buy(1, 10)

		assert.Error(t, err)
	})

	t.Run("success_deducts_stock_and_creates_purchase", func(t *testing.T) {
		activity := runningActivity(1)
		db := &mockSeckillDB{activity: activity}
		svc, mr, _ := newSeckillTestService(t, db)
		mr.Set(seckillpkg.StockKey(1), "2")

		err := svc.Buy(1, 10)

		require.NoError(t, err)
		assertRedisValue(t, mr, seckillpkg.StockKey(1), "1")
		assert.True(t, mr.Exists(seckillpkg.PurchaseKey(1, 10)))
		require.Equal(t, 1, db.createCalls)
		assert.Equal(t, uint(10), db.lastPurchase.UserID)
		assert.Equal(t, uint(1), db.lastPurchase.SeckillActivityID)
		assert.Equal(t, activity.ProductID, db.lastPurchase.ProductID)
		assert.Equal(t, model.SeckillPurchaseCreated, db.lastPurchase.Status)
	})

	t.Run("already_bought", func(t *testing.T) {
		activity := runningActivity(1)
		db := &mockSeckillDB{activity: activity}
		svc, mr, _ := newSeckillTestService(t, db)
		mr.Set(seckillpkg.StockKey(1), "2")

		require.NoError(t, svc.Buy(1, 10))
		err := svc.Buy(1, 10)

		assert.Error(t, err)
		assert.Equal(t, 1, db.createCalls)
		assertRedisValue(t, mr, seckillpkg.StockKey(1), "1")
	})

	t.Run("create_purchase_failed_rolls_back_redis", func(t *testing.T) {
		activity := runningActivity(1)
		db := &mockSeckillDB{activity: activity, createErr: errors.New("db create failed")}
		svc, mr, _ := newSeckillTestService(t, db)
		mr.Set(seckillpkg.StockKey(1), "2")

		err := svc.Buy(1, 10)

		assert.Error(t, err)
		assertRedisValue(t, mr, seckillpkg.StockKey(1), "2")
		assert.False(t, mr.Exists(seckillpkg.PurchaseKey(1, 10)))
	})
}

func TestSeckillServiceCreatePurchaseFromMessage(t *testing.T) {
	t.Run("invalid_message", func(t *testing.T) {
		svc, _, _ := newSeckillTestService(t, &mockSeckillDB{})

		assert.Error(t, svc.CreatePurchaseFromMessage(0, 1))
		assert.Error(t, svc.CreatePurchaseFromMessage(1, 0))
	})

	t.Run("activity_query_failed", func(t *testing.T) {
		svc, _, _ := newSeckillTestService(t, &mockSeckillDB{getErr: gorm.ErrRecordNotFound})

		err := svc.CreatePurchaseFromMessage(1, 10)

		assert.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		activity := runningActivity(1)
		db := &mockSeckillDB{activity: activity}
		svc, _, _ := newSeckillTestService(t, db)

		err := svc.CreatePurchaseFromMessage(1, 10)

		require.NoError(t, err)
		require.Equal(t, 1, db.createCalls)
		assert.Equal(t, uint(10), db.lastPurchase.UserID)
	})

	t.Run("duplicated_key_is_idempotent_success", func(t *testing.T) {
		activity := runningActivity(1)
		db := &mockSeckillDB{activity: activity, createErr: gorm.ErrDuplicatedKey}
		svc, _, _ := newSeckillTestService(t, db)

		err := svc.CreatePurchaseFromMessage(1, 10)

		assert.NoError(t, err)
	})

	t.Run("create_failed_rolls_back_redis", func(t *testing.T) {
		activity := runningActivity(1)
		db := &mockSeckillDB{activity: activity, createErr: errors.New("db create failed")}
		svc, mr, _ := newSeckillTestService(t, db)
		mr.Set(seckillpkg.StockKey(1), "1")
		mr.Set(seckillpkg.PurchaseKey(1, 10), "10")

		err := svc.CreatePurchaseFromMessage(1, 10)

		assert.Error(t, err)
		assertRedisValue(t, mr, seckillpkg.StockKey(1), "2")
		assert.False(t, mr.Exists(seckillpkg.PurchaseKey(1, 10)))
	})
}

func TestSeckillDerivedStatus(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.Local)
	cases := []struct {
		name       string
		status     string
		startAt    time.Time
		endAt      time.Time
		wantStatus string
	}{
		{name: "draft_kept", status: model.SeckillActivityDraft, startAt: now.Add(-time.Hour), endAt: now.Add(time.Hour), wantStatus: model.SeckillActivityDraft},
		{name: "closed_kept", status: model.SeckillActivityClosed, startAt: now.Add(-time.Hour), endAt: now.Add(time.Hour), wantStatus: model.SeckillActivityClosed},
		{name: "before_start_is_upcoming", status: model.SeckillActivityRunning, startAt: now.Add(time.Minute), endAt: now.Add(time.Hour), wantStatus: model.SeckillActivityUpcoming},
		{name: "inside_window_is_running", status: model.SeckillActivityRunning, startAt: now.Add(-time.Minute), endAt: now.Add(time.Hour), wantStatus: model.SeckillActivityRunning},
		{name: "after_end_is_ended_even_if_db_says_running", status: model.SeckillActivityRunning, startAt: now.Add(-2 * time.Hour), endAt: now.Add(-time.Minute), wantStatus: model.SeckillActivityEnded},
		{name: "explicit_ended_kept", status: model.SeckillActivityEnded, startAt: now.Add(-time.Hour), endAt: now.Add(time.Hour), wantStatus: model.SeckillActivityEnded},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			activity := &model.SeckillActivity{Status: tc.status, StartAt: tc.startAt, EndAt: tc.endAt}
			assert.Equal(t, tc.wantStatus, activity.DerivedStatus(now))
		})
	}
}

func TestSeckillServiceBuyRejectsExpiredActivity(t *testing.T) {
	ended := runningActivity(1)
	ended.EndAt = time.Now().Add(-time.Minute)
	svc, mr, _ := newSeckillTestService(t, &mockSeckillDB{activity: ended})
	mr.Set(seckillpkg.StockKey(1), "5")

	err := svc.Buy(1, 10)

	var biz *core.BizError
	assert.ErrorAs(t, err, &biz)
	assert.Equal(t, http.StatusGone, biz.Code)
	assert.Equal(t, "5", mustRedis(t, mr, seckillpkg.StockKey(1)), "已结束活动不能扣减库存")
}

func TestSeckillServiceBuyRejectsUpcomingActivity(t *testing.T) {
	upcoming := runningActivity(2)
	upcoming.StartAt = time.Now().Add(time.Hour)
	upcoming.EndAt = time.Now().Add(2 * time.Hour)
	svc, mr, _ := newSeckillTestService(t, &mockSeckillDB{activity: upcoming})
	mr.Set(seckillpkg.StockKey(2), "5")

	err := svc.Buy(2, 10)

	var biz *core.BizError
	assert.ErrorAs(t, err, &biz)
	assert.Equal(t, http.StatusConflict, biz.Code)
}

func TestSeckillServiceListDerivesStatus(t *testing.T) {
	now := time.Now()
	expired := runningActivity(1)
	expired.EndAt = now.Add(-time.Minute)
	future := runningActivity(2)
	future.StartAt = now.Add(time.Hour)
	future.EndAt = now.Add(2 * time.Hour)

	db := &mockSeckillDB{listActivities: []*model.SeckillActivity{expired, future}}
	svc, _, _ := newSeckillTestService(t, db)
	activities, err := svc.ListActivities()
	require.NoError(t, err)
	require.Len(t, activities, 2)
	assert.Equal(t, model.SeckillActivityEnded, activities[0].Status)
	assert.Equal(t, model.SeckillActivityUpcoming, activities[1].Status)
}

func mustRedis(t *testing.T, mr *miniredis.Miniredis, key string) string {
	t.Helper()
	value, err := mr.Get(key)
	require.NoError(t, err)
	return value
}
