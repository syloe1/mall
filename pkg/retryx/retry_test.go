package retryx

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ─── Do: success ──────────────────────────────────────────────────────────────

func TestDo_Success(t *testing.T) {
	t.Run("first_try", func(t *testing.T) {
		var count int32
		fn := func() error {
			atomic.AddInt32(&count, 1)
			return nil
		}
		err := Do(context.Background(), 3, time.Millisecond, fn)
		assert.NoError(t, err)
		assert.Equal(t, int32(1), count)
	})

	t.Run("after_retries", func(t *testing.T) {
		var count int32
		fn := func() error {
			n := atomic.AddInt32(&count, 1)
			if n < 3 {
				return errors.New("fail")
			}
			return nil
		}
		// baseDelay=1ns 足够小，不会影响测试速度
		err := Do(context.Background(), 5, time.Nanosecond, fn)
		assert.NoError(t, err)
		assert.Equal(t, int32(3), count)
	})
}

// ─── Do: all fail ─────────────────────────────────────────────────────────────

func TestDo_AllFail(t *testing.T) {
	t.Run("attempts_1", func(t *testing.T) {
		var count int32
		testErr := errors.New("always fail")
		err := Do(context.Background(), 1, time.Millisecond, func() error {
			atomic.AddInt32(&count, 1)
			return testErr
		})
		assert.ErrorIs(t, err, testErr)
		assert.Equal(t, int32(1), count)
	})

	t.Run("attempts_3", func(t *testing.T) {
		var count int32
		testErr := errors.New("always fail")
		err := Do(context.Background(), 3, time.Nanosecond, func() error {
			atomic.AddInt32(&count, 1)
			return testErr
		})
		assert.ErrorIs(t, err, testErr)
		assert.Equal(t, int32(3), count)
	})
}

// ─── Do: default params ───────────────────────────────────────────────────────

func TestDo_DefaultParams(t *testing.T) {
	t.Run("attempts_zero_means_one", func(t *testing.T) {
		var count int32
		err := Do(context.Background(), 0, time.Millisecond, func() error {
			atomic.AddInt32(&count, 1)
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, int32(1), count, "attempts=0 should default to 1")
	})
}

// ─── Do: ctx cancelled ────────────────────────────────────────────────────────

func TestDo_CtxCancelled(t *testing.T) {
	t.Run("already_cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		var called int32
		err := Do(ctx, 3, time.Millisecond, func() error {
			atomic.AddInt32(&called, 1)
			return nil
		})
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, int32(0), called, "fn should not be called if ctx already cancelled")
	})

	t.Run("during_wait", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancelCalled := make(chan struct{})

		var count int32
		go func() {
			// 等 fn 第一次执行后立即取消
			for atomic.LoadInt32(&count) == 0 {
				select {
				case <-cancelCalled:
					return
				default:
				}
			}
			cancel()
		}()

		err := Do(ctx, 5, 100*time.Millisecond, func() error {
			atomic.AddInt32(&count, 1)
			return errors.New("fail")
		})
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, int32(1), atomic.LoadInt32(&count), "fn called once, then ctx cancelled during wait")
		close(cancelCalled)
	})
}
