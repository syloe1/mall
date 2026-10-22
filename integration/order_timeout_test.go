//go:build integration
// +build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mall/model"
	"mall/service"
)

// TestOrderCancel — 订单可被 Cancel 方法取消（模拟超时取消）
func TestOrderCancel(t *testing.T) {
	env := NewTestEnv(t)
	defer env.Close()
	env.CleanAll(t)

	// ── 1. 准备用户 ─────────────────────────────────────────────
	require.NoError(t, env.UserService.Register("ordertest", "123456"))
	var user model.User
	env.DB.Where("username = ?", "ordertest").First(&user)
	userID := user.ID

	// ── 2. 准备商品 ─────────────────────────────────────────────
	productID := env.seedProduct(t)

	// ── 3. 准备地址 ─────────────────────────────────────────────
	addr := &model.UserAddress{
		UserID:       userID,
		ReceiverName: "李四",
		Phone:        "13900139000",
		Province:     "北京",
		City:         "北京",
		District:     "朝阳",
		Detail:       "望京",
	}
	require.NoError(t, env.DB.Create(addr).Error)

	// ── 4. 加购物车 + 勾选 ──────────────────────────────────────
	require.NoError(t, env.CartService.AddItem(service.AddCartItemInput{
		UserID:    userID,
		ProductID: productID,
		Quantity:  1,
	}))
	env.DB.Model(&model.CartItem{}).Where("user_id = ?", userID).Update("checked", true)

	// ── 5. 下单 → 订单状态 pending_pay ───────────────────────────
	order, err := env.OrderService.CreateFromCart(service.CreateOrderFromCartInput{
		UserID:    userID,
		AddressID: addr.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, model.OrderPendingPay, order.Status)

	// ── 6. 模拟超时取消（30分钟后由 RabbitMQ 延迟消息触发） ──────
	//    直接调 CancelTimeoutOrder 模拟 MQ 消费者收到超时消息
	err = env.OrderService.CancelTimeoutOrder(userID, order.ID)
	require.NoError(t, err, "cancel timeout order failed")

	// ── 7. 验证订单已取消 ────────────────────────────────────────
	var cancelled model.Order
	env.DB.First(&cancelled, order.ID)
	assert.Equal(t, model.OrderCanceled, cancelled.Status)

	// ── 8. 再次取消应无影响（rows=0 但不报错） ────────────────────
	err = env.OrderService.CancelTimeoutOrder(userID, order.ID)
	assert.NoError(t, err) // 幂等

	t.Logf("✅ 订单取消流程通过: order=%d, time=%s", order.ID, time.Now().Format(time.RFC3339))
}
