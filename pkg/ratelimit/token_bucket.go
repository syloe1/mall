package ratelimit

import (
	"sync"
	"time"
)

type TokenBucket struct {
	mu       sync.Mutex
	rate     float64
	capacity float64
	tokens   float64
	last     time.Time
}

func NewTokenBucket(ratePerSecond int, capacity int) *TokenBucket {
	if ratePerSecond <= 0 {
		ratePerSecond = 100
	}
	if capacity <= 0 {
		capacity = ratePerSecond
	}
	return &TokenBucket{
		rate:     float64(ratePerSecond),
		capacity: float64(capacity),
		tokens:   float64(capacity),
		last:     time.Now(),
	}
}

func (b *TokenBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * b.rate
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	b.last = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
