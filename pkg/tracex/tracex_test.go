package tracex

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"mall/config"
)

func TestInit(t *testing.T) {
	ctx := context.Background()

	t.Run("disabled", func(t *testing.T) {
		shutdown, err := Init(ctx, config.TraceConfig{
			Enabled: false,
		})

		assert.NoError(t, err)
		assert.NotNil(t, shutdown, "shutdown func should not be nil even when disabled")

		// 调用 shutdown 不应该报错
		assert.NoError(t, shutdown(ctx))
	})

	t.Run("enabled_with_defaults", func(t *testing.T) {
		shutdown, err := Init(ctx, config.TraceConfig{
			Enabled:        true,
			ServiceName:    "",
			JaegerEndpoint: "",
		})

		assert.NoError(t, err)
		assert.NotNil(t, shutdown)

		// 调用 shutdown 清理资源
		assert.NoError(t, shutdown(ctx))
	})

	t.Run("enabled_with_custom_values", func(t *testing.T) {
		shutdown, err := Init(ctx, config.TraceConfig{
			Enabled:        true,
			ServiceName:    "my-test-svc",
			JaegerEndpoint: "http://localhost:14268/api/traces",
		})

		assert.NoError(t, err)
		assert.NotNil(t, shutdown)

		assert.NoError(t, shutdown(ctx))
	})
}
