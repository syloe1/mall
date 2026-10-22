package seckill

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

var stockScript = redis.NewScript(`
local stockKey = KEYS[1]
local purchaseKey = KEYS[2]

local userID = ARGV[1]
local ttl = tonumber(ARGV[2])

if redis.call("EXISTS", purchaseKey) == 1 then
	return -1
end

local stock = tonumber(redis.call("GET", stockKey) or "0")
if stock <= 0 then
	return 0
end

redis.call("DECR", stockKey)
redis.call("SET", purchaseKey, userID, "EX", ttl)

return 1
`)

var ErrStockAlreadyInitialized = errors.New("seckill stock already initialized")

// 库存 key：seckill:stock:101
func StockKey(activityID uint) string {
	return fmt.Sprintf("seckill:stock:%d", activityID)
}

// 用户购买记录 key：seckill:buy:101:2001
func PurchaseKey(activityID, userID uint) string {
	return fmt.Sprintf("seckill:buy:%d:%d", activityID, userID)
}

// TryDeduct 尝试扣库存
func TryDeduct(ctx context.Context, rdb *redis.Client, activityID, userID uint, ttl time.Duration) (int64, error) {
	return stockScript.Run(ctx, rdb, []string{StockKey(activityID), PurchaseKey(activityID, userID)}, userID, int64(ttl.Seconds())).Int64()
}

// 订单超时未支付 用户取消订单 支付失败
func RollbackDeduct(ctx context.Context, rdb *redis.Client, activityID, userID uint) error {
	pipe := rdb.TxPipeline()
	pipe.Incr(ctx, StockKey(activityID))
	pipe.Del(ctx, PurchaseKey(activityID, userID))
	_, err := pipe.Exec(ctx)
	return err
}

// InitStock 初始化秒杀库存
func InitStock(ctx context.Context, rdb *redis.Client, activityID uint, stock int, ttl time.Duration) error {
	if stock < 0 {
		return errors.New("stock cannot be negative")
	}

	ok, err := rdb.SetNX(ctx, StockKey(activityID), stock, ttl).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrStockAlreadyInitialized
	}
	return nil
}

//1. 后台创建秒杀活动 → InitStock 初始化库存
//2. 用户点击抢购 → TryDeduct 扣库存
//   返回 1 → 成功
//   返回 0 → 没库存
//   返回 -1 → 重复购买
//3. 下单成功 → 等待支付
//4. 15分钟未支付 → 调用 RollbackDeduct 回滚库存
