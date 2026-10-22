//go:build integration
// +build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mall/model"
	"mall/service"
)

// TestPurchaseFlow — 注册→登录→创建商品→加购物车→下单→支付
func TestPurchaseFlow(t *testing.T) {
	env := NewTestEnv(t)
	defer env.Close()
	env.CleanAll(t)

	// ── 1. 注册用户 ─────────────────────────────────────────────
	err := env.UserService.Register("buyer", "123456")
	require.NoError(t, err, "register failed")

	// ── 2. 登录 ─────────────────────────────────────────────────
	token, _, role, err := env.UserService.Login("buyer", "123456")
	require.NoError(t, err, "login failed")
	assert.NotEmpty(t, token)
	assert.Equal(t, model.RoleUser, role)

	// Get user ID
	var user model.User
	env.DB.Where("username = ?", "buyer").First(&user)
	require.NotZero(t, user.ID)
	userID := user.ID

	// ── 3. 创建商品 ─────────────────────────────────────────────
	err = env.ProductService.Create(service.CreateProductInput{
		Name:        "测试手机",
		PriceCents:  299900,
		Description: "一部测试手机",
		Stock:       50,
		CategoryID:  1,
		Status:      model.ProductOnSale,
		Images:      []string{"https://img.example.com/phone.jpg"},
	})
	require.NoError(t, err, "create product failed")

	// Get product ID
	var product model.Product
	env.DB.Where("name = ?", "测试手机").First(&product)
	require.NotZero(t, product.ID)
	productID := product.ID

	// ── 4. 创建收货地址 ──────────────────────────────────────────
	addr := &model.UserAddress{
		UserID:       userID,
		ReceiverName: "张三",
		Phone:        "13800138000",
		Province:     "广东",
		City:         "深圳",
		District:     "南山",
		Detail:       "科技园 1 栋",
		PostalCode:   "518000",
	}
	require.NoError(t, env.DB.Create(addr).Error)

	// ── 5. 加入购物车 ───────────────────────────────────────────
	err = env.CartService.AddItem(service.AddCartItemInput{
		UserID:    userID,
		ProductID: productID,
		Quantity:  2,
	})
	require.NoError(t, err, "add to cart failed")

	// ── 6. 勾选购物车商品 ───────────────────────────────────────
	var cartItem model.CartItem
	env.DB.Where("user_id = ? AND product_id = ?", userID, productID).First(&cartItem)

	_, err = env.DB.Model(&cartItem).Update("checked", true).Rows()
	require.NoError(t, err)

	// ── 7. 购物车结算下单 ───────────────────────────────────────
	order, err := env.OrderService.CreateFromCart(service.CreateOrderFromCartInput{
		UserID:    userID,
		AddressID: addr.ID,
	})
	require.NoError(t, err, "create order from cart failed")
	assert.Equal(t, model.OrderPendingPay, order.Status)
	assert.Equal(t, int64(299900*2), order.TotalAmount)
	assert.Len(t, order.Items, 1)
	assert.Equal(t, productID, order.Items[0].ProductID)
	assert.Equal(t, 2, order.Items[0].Quantity)

	// ── 8. 验证库存扣减 ─────────────────────────────────────────
	var updatedProduct model.Product
	env.DB.First(&updatedProduct, productID)
	assert.Equal(t, 48, updatedProduct.Stock) // 50 - 2

	// ── 9. 创建支付（模拟支付） ──────────────────────────────────
	payment, err := env.PaymentService.CreatePayment(service.CreatePaymentInput{
		UserID:  userID,
		OrderID: order.ID,
		Channel: model.PaymentChannelMock,
	})
	require.NoError(t, err, "create payment failed")
	assert.Equal(t, model.PaymentStatusPending, payment.Status)

	// ── 10. 模拟支付成功 ────────────────────────────────────────
	paid, err := env.PaymentService.MockPaySuccess(service.MockPaySuccessInput{
		UserID:    userID,
		PaymentNo: payment.PaymentNo,
	})
	require.NoError(t, err, "mock pay success failed")
	assert.Equal(t, model.PaymentStatusSuccess, paid.Status)

	// ── 11. 验证订单状态变为已支付 ───────────────────────────────
	var paidOrder model.Order
	env.DB.First(&paidOrder, order.ID)
	assert.Equal(t, model.OrderPaid, paidOrder.Status)

	t.Logf("✅ 完整购买流程通过: user=%d product=%d order=%d payment=%s", userID, productID, order.ID, payment.PaymentNo)
}
