package ratelimit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ─── NewTokenBucket ───────────────────────────────────────────────────────────

func TestNewTokenBucket(t *testing.T) {
	t.Run("normal_params", func(t *testing.T) {
		b := NewTokenBucket(10, 20)
		assert.Equal(t, float64(10), b.rate)
		assert.Equal(t, float64(20), b.capacity)
		assert.Equal(t, float64(20), b.tokens)
		assert.False(t, b.last.IsZero())
	})

	t.Run("rate_zero_defaults_to_100", func(t *testing.T) {
		b := NewTokenBucket(0, 50)
		assert.Equal(t, float64(100), b.rate)
		assert.Equal(t, float64(50), b.capacity)
		assert.Equal(t, float64(50), b.tokens)
	})

	t.Run("rate_negative_defaults_to_100", func(t *testing.T) {
		b := NewTokenBucket(-5, 30)
		assert.Equal(t, float64(100), b.rate)
		assert.Equal(t, float64(30), b.capacity)
	})

	t.Run("capacity_zero_defaults_to_rate", func(t *testing.T) {
		b := NewTokenBucket(25, 0)
		assert.Equal(t, float64(25), b.rate)
		assert.Equal(t, float64(25), b.capacity)
		assert.Equal(t, float64(25), b.tokens)
	})

	t.Run("both_invalid_both_default_to_100", func(t *testing.T) {
		b := NewTokenBucket(-1, -1)
		assert.Equal(t, float64(100), b.rate)
		assert.Equal(t, float64(100), b.capacity)
		assert.Equal(t, float64(100), b.tokens)
	})
}

// ─── Allow ────────────────────────────────────────────────────────────────────

func TestAllow_Initial(t *testing.T) {
	b := NewTokenBucket(10, 5)
	// 初始 capacity=5 个 token，前 5 次应该 true
	for i := 0; i < 5; i++ {
		assert.True(t, b.Allow(), "allow %d should be true", i+1)
	}
	// 第 6 次，token 已耗尽
	assert.False(t, b.Allow())
}

func TestAllow_ExhaustThenRefill(t *testing.T) {
	b := NewTokenBucket(10, 3)
	// 消耗全部 3 个 token
	assert.True(t, b.Allow())
	assert.True(t, b.Allow())
	assert.True(t, b.Allow())
	assert.False(t, b.Allow())

	// 模拟时间倒退 1 秒，rate=10 → 增加 10 个 token
	b.last = time.Now().Add(-1 * time.Second)
	// 但 capacity=3，所以最多回到 3
	assert.True(t, b.Allow())
	assert.True(t, b.Allow())
	assert.True(t, b.Allow())
	assert.False(t, b.Allow())
}

func TestAllow_RefillLimitedByCapacity(t *testing.T) {
	b := NewTokenBucket(100, 2)
	assert.True(t, b.Allow())
	assert.True(t, b.Allow())
	assert.False(t, b.Allow())

	// 模拟时间倒退 10 秒，rate=100/s → 1000 tokens，但 capacity=2 限制
	b.last = time.Now().Add(-10 * time.Second)
	assert.True(t, b.Allow())
	assert.True(t, b.Allow())
	assert.False(t, b.Allow())
}

func TestAllow_PartialRefill(t *testing.T) {
	b := NewTokenBucket(10, 10)
	// 消耗 5 个
	for i := 0; i < 5; i++ {
		assert.True(t, b.Allow())
	}

	// 模拟 0.1 秒后，rate=10 → +1 token
	b.last = time.Now().Add(-100 * time.Millisecond)
	assert.True(t, b.Allow()) // 补充的 1 个被消耗，还剩 ~5
	// 因时间流逝 < 0.1s，不足以补充 1 个，继续消耗之前的 5 个
	assert.True(t, b.Allow())
	assert.True(t, b.Allow())
	assert.True(t, b.Allow())
	assert.True(t, b.Allow())
	assert.True(t, b.Allow())
	assert.False(t, b.Allow())
}
