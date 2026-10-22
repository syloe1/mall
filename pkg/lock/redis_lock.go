package lock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
)

var ErrLockNotAcquired = errors.New("lock not acquired")

type RedisLock struct {
	client *redis.Client // redis 客户端
	key    string        // 锁的名字（比如：stock:1001）
	value  string        // 随机值（防止误删别人的锁！）
	ttl    time.Duration // 锁过期时间（防止死锁）
}

func NewRedisLock(client *redis.Client, key string, ttl time.Duration) *RedisLock {
	return &RedisLock{
		client: client,
		key:    key,
		value:  randomValue(), // 生成唯一随机串
		ttl:    ttl,           // 锁自动过期时间
	}
}

func (l *RedisLock) Lock(ctx context.Context) error {
	// SetNX = set if not exists（不存在才设置）
	// 这就是“抢锁”
	ok, err := l.client.SetNX(ctx, l.key, l.value, l.ttl).Result()

	if err != nil {
		return err // redis挂了
	}
	if !ok {
		return ErrLockNotAcquired // 没抢到
	}
	return nil // 抢到锁！
}
func (l *RedisLock) Unlock(ctx context.Context) error {
	const script = `
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
else
	return 0
end
	`
	return l.client.Eval(ctx, script, []string{l.key}, l.value).Err()
}

func randomValue() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
