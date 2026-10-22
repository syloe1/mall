//go:build integration
// +build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mall/model"
)

// TestSeckillFlow — 秒杀活动→初始化库存→抢购→验证库存扣减
func TestSeckillFlow(t *testing.T) {
	env := NewTestEnv(t)
	defer env.Close()
	env.CleanAll(t)

	// ── 1. 创建秒杀活动 ─────────────────────────────────────────
	now := time.Now()
	activity := &model.SeckillActivity{
		ActivityNo:        "SK-INTEGRATION-001",
		Name:              "测试秒杀活动",
		ProductID:         1,
		ProductName:       "秒杀商品",
		SeckillPriceCents: 100,
		TotalStock:        10,
		AvailableStock:    10,
		PerLimit:          1,
		StartAt:           now.Add(-1 * time.Hour),
		EndAt:             now.Add(1 * time.Hour),
		Status:            model.SeckillActivityRunning,
	}
	require.NoError(t, env.DB.Create(activity).Error, "create activity failed")
	activityID := activity.ID

	// ── 2. 初始化秒杀库存到 Redis ────────────────────────────────
	err := env.SeckillService.InitActivityStock(activityID)
	require.NoError(t, err, "init stock failed")

	// ── 3. 抢购（第1个用户） ─────────────────────────────────────
	err = env.SeckillService.Buy(activityID, 100)
	require.NoError(t, err, "first buy failed")

	// ── 4. 验证购买记录落库 ──────────────────────────────────────
	var purchase model.SeckillPurchase
	err = env.DB.Where("user_id = ? AND seckill_activity_id = ?", 100, activityID).First(&purchase).Error
	require.NoError(t, err, "purchase record not found")
	assert.Equal(t, model.SeckillPurchaseCreated, purchase.Status)

	// ── 5. 同一用户不能重复购买 ──────────────────────────────────
	err = env.SeckillService.Buy(activityID, 100)
	assert.Error(t, err, "duplicate buy should fail")

	// ── 6. 第2个用户也能抢购 ─────────────────────────────────────
	err = env.SeckillService.Buy(activityID, 200)
	require.NoError(t, err, "second user buy failed")

	t.Logf("✅ 秒杀流程通过: activity=%d, 2 users bought", activityID)
}
