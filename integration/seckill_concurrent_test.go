//go:build integration
// +build integration

package integration

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mall/model"
)

// TestSeckillConcurrent — 并发秒杀不超卖：50 人抢 10 件商品，最终只应有 10 人成功
func TestSeckillConcurrent(t *testing.T) {
	env := NewTestEnv(t)
	defer env.Close()
	env.CleanAll(t)

	const (
		totalStock    = 10
		concurrentUsers = 50
	)

	// ── 1. 创建秒杀活动（10 件库存） ──────────────────────────────
	now := time.Now()
	activity := &model.SeckillActivity{
		ActivityNo:        "SK-CONCURRENT-001",
		Name:              "并发秒杀测试",
		ProductID:         1,
		ProductName:       "限量商品",
		SeckillPriceCents: 1,
		TotalStock:        totalStock,
		AvailableStock:    totalStock,
		PerLimit:          1,
		StartAt:           now.Add(-1 * time.Hour),
		EndAt:             now.Add(1 * time.Hour),
		Status:            model.SeckillActivityRunning,
	}
	require.NoError(t, env.DB.Create(activity).Error)
	activityID := activity.ID

	// ── 2. 初始化 Redis 库存 ──────────────────────────────────────
	require.NoError(t, env.SeckillService.InitActivityStock(activityID))

	// ── 3. 并发抢购 ──────────────────────────────────────────────
	var wg sync.WaitGroup
	results := make(chan bool, concurrentUsers) // true=成功, false=失败

	for i := uint(1); i <= concurrentUsers; i++ {
		wg.Add(1)
		go func(userID uint) {
			defer wg.Done()
			err := env.SeckillService.Buy(activityID, userID)
			results <- (err == nil)
		}(i)
	}

	wg.Wait()
	close(results)

	// ── 4. 统计结果 ──────────────────────────────────────────────
	successCount := 0
	failCount := 0
	for success := range results {
		if success {
			successCount++
		} else {
			failCount++
		}
	}

	t.Logf("并发 %d 人抢 %d 件: 成功=%d, 失败=%d", concurrentUsers, totalStock, successCount, failCount)

	// ── 5. 验证不超卖 ────────────────────────────────────────────
	assert.Equal(t, totalStock, successCount,
		"应恰好 %d 人成功，实际 %d 人成功（超卖了！）", totalStock, successCount)

	// ── 6. 验证数据库中的购买记录数 ──────────────────────────────
	var count int64
	env.DB.Model(&model.SeckillPurchase{}).
		Where("seckill_activity_id = ?", activityID).
		Count(&count)
	assert.Equal(t, int64(totalStock), count,
		"数据库应有 %d 条购买记录，实际 %d 条", totalStock, count)

	// ── 7. 验证 Redis 库存已扣减到 0 ──────────────────────────────
	// 发起第 51 个请求，应返回 sold out
	err := env.SeckillService.Buy(activityID, 999)
	assert.Error(t, err, "库存耗尽后应拒绝购买")

	// ── 8. 验证没有同一用户重复购买 ────────────────────────────────
	// 让前 totalStock 个成功用户再试一次，都应被拒绝
	var successUsers []uint
	rows, _ := env.DB.Model(&model.SeckillPurchase{}).
		Where("seckill_activity_id = ?", activityID).
		Rows()
	defer rows.Close()
	for rows.Next() {
		var p model.SeckillPurchase
		env.DB.ScanRows(rows, &p)
		successUsers = append(successUsers, p.UserID)
	}

	for _, uid := range successUsers[:5] { // 抽查前 5 个
		err := env.SeckillService.Buy(activityID, uid)
		assert.Error(t, err, "用户 %d 重复购买应被拒绝", uid)
	}

	t.Logf("✅ 并发秒杀不超卖通过: %d人抢%d件, 成功%d, 数据库记录%d",
		concurrentUsers, totalStock, successCount, count)
}
