package mq

import (
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
)

func newCarrier() amqpHeaderCarrier {
	return amqpHeaderCarrier{}
}

// ─── Get ──────────────────────────────────────────────────────────────────────

func TestAmqpHeaderCarrier_Get(t *testing.T) {
	t.Run("existing_string", func(t *testing.T) {
		c := amqpHeaderCarrier{"trace-id": "abc123"}
		assert.Equal(t, "abc123", c.Get("trace-id"))
	})

	t.Run("non_existing_key", func(t *testing.T) {
		c := newCarrier()
		assert.Equal(t, "", c.Get("not-exist"))
	})

	t.Run("nil_value", func(t *testing.T) {
		c := amqpHeaderCarrier{"key": nil}
		assert.Equal(t, "", c.Get("key"))
	})

	t.Run("non_string_type", func(t *testing.T) {
		c := amqpHeaderCarrier{"key": 42}
		assert.Equal(t, "", c.Get("key"))
	})
}

// ─── Set ──────────────────────────────────────────────────────────────────────

func TestAmqpHeaderCarrier_Set(t *testing.T) {
	t.Run("set_and_get", func(t *testing.T) {
		c := newCarrier()
		c.Set("span-id", "span-001")
		assert.Equal(t, "span-001", c.Get("span-id"))
	})

	t.Run("overwrite", func(t *testing.T) {
		c := amqpHeaderCarrier{"key": "old"}
		c.Set("key", "new")
		assert.Equal(t, "new", c.Get("key"))
	})

	t.Run("empty_value", func(t *testing.T) {
		c := newCarrier()
		c.Set("key", "")
		assert.Equal(t, "", c.Get("key"))
	})
}

// ─── Keys ─────────────────────────────────────────────────────────────────────

func TestAmqpHeaderCarrier_Keys(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		c := newCarrier()
		assert.Empty(t, c.Keys())
	})

	t.Run("single_key", func(t *testing.T) {
		c := amqpHeaderCarrier{"traceparent": "data"}
		keys := c.Keys()
		assert.Len(t, keys, 1)
		assert.Contains(t, keys, "traceparent")
	})

	t.Run("multiple_keys", func(t *testing.T) {
		c := amqpHeaderCarrier{
			"traceparent": "data1",
			"tracestate":  "data2",
			"span-id":     "data3",
		}
		keys := c.Keys()
		assert.Len(t, keys, 3)
		assert.Contains(t, keys, "traceparent")
		assert.Contains(t, keys, "tracestate")
		assert.Contains(t, keys, "span-id")
	})
}

// ─── Round-trip: Set → Get ────────────────────────────────────────────────────

func TestAmqpHeaderCarrier_RoundTrip(t *testing.T) {
	c := newCarrier()
	c.Set("trace-id", "xyz")
	c.Set("span-id", "abc")

	assert.Equal(t, "xyz", c.Get("trace-id"))
	assert.Equal(t, "abc", c.Get("span-id"))
	assert.Len(t, c.Keys(), 2)
}

// ─── Implements amqp.Table ────────────────────────────────────────────────────

func TestAmqpHeaderCarrier_IsAmqpTable(t *testing.T) {
	c := amqpHeaderCarrier{"x": "y"}
	var table amqp.Table = amqp.Table(c)
	assert.Equal(t, "y", table["x"])
}
