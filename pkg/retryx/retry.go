package retryx

import (
	"context"
	"time"
)

// 执行一段逻辑，如果失败，自动重试多次，每次等待时间翻倍
func Do(ctx context.Context, attempts int, baseDelay time.Duration, fn func() error) error {
	if attempts <= 0 {
		attempts = 1
	}
	if baseDelay <= 0 {
		baseDelay = 100 * time.Millisecond
	}

	var err error
	delay := baseDelay // 第一次等待：baseDelay

	for i := 0; i < attempts; i++ { // 循环 attempts 次
		// 如果 ctx 取消/超时，直接退出
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// 执行业务函数
		if err = fn(); err == nil {
			return nil // 成功！直接返回
		}

		// 如果失败了，且不是最后一次，就等待
		if i < attempts-1 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}

			// 指数退避：下次等待时间 ×2
			delay *= 2
		}
	}

	// 全部重试完还是失败 → 返回最后一次错误
	return err
}
