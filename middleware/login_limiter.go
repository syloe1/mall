package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"

	"mall/pkg/core"
)

// 登录 / 注册限流：IP + 账号双维度固定窗口计数。
//
// 目标（审计要求）：
//   - 5 次/分钟后返回 429，并带 Retry-After: 60
//   - 同时限制同一 IP 与同一账号，避免撞库/爆破
//
// Redis 不可用时降级为放行（可用性优先，且本机已有 nginx 层限流兜底）。
type LoginLimiter struct {
	rdb    *redis.Client
	limit  int
	window time.Duration
}

func NewLoginLimiter(rdb *redis.Client) *LoginLimiter {
	return &LoginLimiter{rdb: rdb, limit: 5, window: time.Minute}
}

// Check 在真正校验密码前调用。超限时直接写 429 响应并返回 false。
func (l *LoginLimiter) Check(c *gin.Context, account string) bool {
	if l == nil || l.rdb == nil {
		return true
	}
	account = strings.ToLower(strings.TrimSpace(account))
	if account == "" {
		account = "-"
	}
	clientIP := c.ClientIP()
	keys := []struct {
		key   string
		limit int
	}{
		{key: "rl:login:ip:" + clientIP, limit: l.limit * 2}, // IP 维度放宽一倍，避免同一出口 NAT 误伤
		{key: "rl:login:account:" + account, limit: l.limit},
	}
	for _, item := range keys {
		count, err := l.increment(c.Request.Context(), item.key)
		if err != nil {
			return true // Redis 抖动时不阻断登录
		}
		if count > int64(item.limit) {
			c.Header("Retry-After", strconv.Itoa(int(l.window.Seconds())))
			c.Header("X-RateLimit-Limit", strconv.Itoa(item.limit))
			c.Header("X-RateLimit-Remaining", "0")
			core.Respond(c, http.StatusTooManyRequests, core.MsgTooMany, nil, nil)
			c.Abort()
			return false
		}
	}
	return true
}

func (l *LoginLimiter) increment(ctx context.Context, key string) (int64, error) {
	pipe := l.rdb.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, l.window)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

// 便于测试时构造更小窗口的限流器
func NewLoginLimiterWithConfig(rdb *redis.Client, limit int, window time.Duration) *LoginLimiter {
	if limit <= 0 {
		limit = 5
	}
	if window <= 0 {
		window = time.Minute
	}
	return &LoginLimiter{rdb: rdb, limit: limit, window: window}
}
