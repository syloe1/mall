package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mall/dao"
	"mall/model"
	"mall/pkg/core"
	"mall/pkg/lock"
	"mall/pkg/mq"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// OrderTimeoutProducer 发延迟关单消息的抽象。
// *mq.Producer 天然满足；抽成接口是为了能在测试里注入 fake —— 之前这里写的是具体类型
// *mq.Producer，导致 publishOrderTimeout 的非 nil 分支无法测试，只能测出「传 nil 就静默不发」。
type OrderTimeoutProducer interface {
	PublishDelayJSON(ctx context.Context, routingKey string, v any, msgID string, delay time.Duration) error
}

// 真实生产者必须始终满足这个接口（web 和 worker 两个进程都传的是 *mq.Producer）
var _ OrderTimeoutProducer = (*mq.Producer)(nil)

type OrderService struct {
	db        *gorm.DB
	rdb       *redis.Client
	producer  OrderTimeoutProducer
	orderDB   dao.OrderDB
	cartDB    dao.CartDB
	productDB dao.ProductDB
	addressDB dao.AddressDB
}

func NewOrderService(db *gorm.DB, rdb *redis.Client, producer OrderTimeoutProducer, orderDB dao.OrderDB, cartDB dao.CartDB, productDB dao.ProductDB, addressDB dao.AddressDB) *OrderService {
	return &OrderService{
		db:        db,
		rdb:       rdb,
		producer:  producer,
		orderDB:   orderDB,
		cartDB:    cartDB,
		productDB: productDB,
		addressDB: addressDB,
	}
}

// 从购物车结算 → 创建订单
type CreateOrderFromCartInput struct {
	UserID    uint
	AddressID uint
}

type BuyNowInput struct {
	UserID    uint
	ProductID uint
	Quantity  uint
	AddressID uint
}

func (s *OrderService) CreateFromCart(input CreateOrderFromCartInput) (*model.Order, error) {
	if input.UserID == 0 {
		return nil, core.NewBizError(http.StatusBadRequest, "用户 ID 不合法")
	}
	if input.AddressID == 0 {
		return nil, core.NewBizError(http.StatusBadRequest, "地址 ID 不合法")
	}

	address, err := s.getAddress(input.UserID, input.AddressID)
	if err != nil {
		return nil, err
	}
	//加锁创建订单（防止重复下单）
	order, err := s.createOrderWithLock(input.UserID, func() (*model.Order, error) {
		var created *model.Order
		/*
			查购物车勾选商品
			构建订单
			扣减库存
			创建订单记录
			删除已结算购物车
		*/
		err := s.orderDB.Transaction(func(txOrderDB dao.OrderDB, txProductDB dao.ProductDB, txCartDB dao.CartDB) error {
			cartItems, err := txCartDB.ListCheckedItems(input.UserID)
			if err != nil {
				return err
			}
			if len(cartItems) == 0 {
				return core.NewBizError(http.StatusBadRequest, "请先勾选要结算的商品")
			}
			//购物车商品转成订单需要的格式
			items := make([]orderBuildItem, 0, len(cartItems))
			for _, cartItem := range cartItems {
				if cartItem.Quantity <= 0 {
					return core.NewBizError(http.StatusBadRequest, "商品数量不合法")
				}
				items = append(items, orderBuildItem{
					ProductID: cartItem.ProductID,
					Quantity:  cartItem.Quantity,
				})
			}
			//. 构建订单（计算价格、生成订单号、填地址等）
			order, err := s.buildOrder(input.UserID, address, items, txProductDB)
			if err != nil {
				return err
			}
			//扣减商品库存（超重要！防止超卖）
			for _, item := range order.Items {
				rows, err := txProductDB.DecreaseStock(item.ProductID, item.Quantity)
				if err != nil {
					return err
				}
				if rows == 0 {
					return core.NewBizError(http.StatusBadRequest, "库存不足")
				}
			}
			//把订单写入数据库
			if err := txOrderDB.Create(order); err != nil {
				return err
			}
			//清空已结算的购物车商品
			for _, item := range cartItems {
				if _, err := txCartDB.DeleteItem(input.UserID, item.ProductID); err != nil {
					return err
				}
			}

			created = order
			return nil
		})
		return created, err
	})
	if err != nil {
		return nil, err
	}
	//发布订单超时取消消息（尽力而为，失败不影响下单结果）
	s.publishOrderTimeout(order)
	return order, nil
}

func (s *OrderService) BuyNow(input BuyNowInput) (*model.Order, error) {
	if input.UserID == 0 {
		return nil, core.NewBizError(http.StatusBadRequest, "用户 ID 不合法")
	}
	if input.ProductID == 0 {
		return nil, core.NewBizError(http.StatusBadRequest, "商品 ID 不合法")
	}
	if input.Quantity == 0 {
		return nil, core.NewBizError(http.StatusBadRequest, "数量必须大于 0")
	}
	if input.AddressID == 0 {
		return nil, core.NewBizError(http.StatusBadRequest, "地址 ID 不合法")
	}

	address, err := s.getAddress(input.UserID, input.AddressID)
	if err != nil {
		return nil, err
	}
	//加锁防重复下单
	order, err := s.createOrderWithLock(input.UserID, func() (*model.Order, error) {
		var created *model.Order
		/*
			构建订单
			扣减库存
			创建订单
			提交
		*/
		err := s.orderDB.Transaction(func(txOrderDB dao.OrderDB, txProductDB dao.ProductDB, txCartDB dao.CartDB) error {
			order, err := s.buildOrder(input.UserID, address, []orderBuildItem{
				{
					ProductID: input.ProductID,
					Quantity:  int(input.Quantity),
				},
			}, txProductDB)
			if err != nil {
				return err
			}
			for _, item := range order.Items {
				//扣减库存（防超卖）
				rows, err := txProductDB.DecreaseStock(item.ProductID, item.Quantity)
				if err != nil {
					return err
				}
				if rows == 0 {
					return core.NewBizError(http.StatusBadRequest, "库存不足")
				}
			}
			//创建订单到数据库
			if err := txOrderDB.Create(order); err != nil {
				return err
			}

			created = order
			return nil
		})
		return created, err
	})
	if err != nil {
		return nil, err
	}
	// 发布订单超时消息（尽力而为，失败不影响下单结果）
	s.publishOrderTimeout(order)
	return order, nil
}

// CreateSeckillOrder 秒杀消费者调用：按活动秒杀价生成一笔待支付订单（库存已由 Redis 预扣，这里不再扣商品库存）
func (s *OrderService) CreateSeckillOrder(userID uint, activity *model.SeckillActivity) (*model.Order, error) {
	if userID == 0 || activity == nil {
		return nil, core.NewBizError(http.StatusBadRequest, "秒杀订单参数不合法")
	}
	address, err := s.addressDB.GetDefault(userID)
	if err != nil {
		return nil, core.NewBizError(http.StatusBadRequest, "请先在「收货地址」添加默认地址再参与秒杀")
	}
	qty := activity.PerLimit
	if qty <= 0 {
		qty = 1
	}
	price := activity.SeckillPriceCents
	subtotal := price * int64(qty)

	order := &model.Order{
		OrderNo:      generateOrderNo(userID),
		UserID:       userID,
		Status:       model.OrderPendingPay,
		ReceiverName: address.ReceiverName,
		Phone:        address.Phone,
		Province:     address.Province,
		City:         address.City,
		District:     address.District,
		Detail:       address.Detail,
		PostalCode:   address.PostalCode,
		TotalAmount:  subtotal,
		Items: []model.OrderItem{
			{
				ProductID:    activity.ProductID,
				ProductName:  activity.ProductName,
				ProductImage: activity.ProductImage,
				PriceCents:   price,
				Quantity:     qty,
				Subtotal:     subtotal,
			},
		},
	}
	if err := s.orderDB.Create(order); err != nil {
		return nil, err
	}
	// 30 分钟未支付自动关单（尽力而为，失败不影响建单结果）
	s.publishOrderTimeout(order)
	return order, nil
}

type orderBuildItem struct {
	ProductID uint
	Quantity  int
}

func (s *OrderService) getAddress(userID uint, addressID uint) (*model.UserAddress, error) {
	address, err := s.addressDB.GetByID(userID, addressID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.NewBizError(http.StatusNotFound, "收货地址不存在")
		}
		return nil, err
	}
	return address, nil
}

func (s *OrderService) createOrderWithLock(userID uint, fn func() (*model.Order, error)) (*model.Order, error) {
	// 如果没有 Redis，就不加锁（直接执行）
	if s.rdb == nil {
		return fn()
	}

	ctx := context.Background()
	//Redis 锁的 key（按用户加锁）
	l := lock.NewRedisLock(s.rdb, fmt.Sprintf("lock:order:create:user:%d", userID), 5*time.Second)
	if err := l.Lock(ctx); err != nil {
		if errors.Is(err, lock.ErrLockNotAcquired) {
			// 没拿到锁 → 说明用户在重复提交
			return nil, core.NewBizError(http.StatusTooManyRequests, "订单提交过于频繁，请稍后再试")
		}
		return nil, err
	}
	defer func() { _ = l.Unlock(ctx) }()

	return fn()
}

func (s *OrderService) buildOrder(userID uint, address *model.UserAddress, buildItems []orderBuildItem, productDB dao.ProductDB) (*model.Order, error) {
	order := &model.Order{
		OrderNo:      generateOrderNo(userID), // 生成唯一订单号
		UserID:       userID,                  // 下单用户
		Status:       model.OrderPendingPay,   // 状态：待支付
		ReceiverName: address.ReceiverName,    // 收货人
		Phone:        address.Phone,           // 手机
		Province:     address.Province,        // 省
		City:         address.City,            // 市
		District:     address.District,        // 区
		Detail:       address.Detail,          // 详细地址
		PostalCode:   address.PostalCode,      // 邮编
		Items:        make([]model.OrderItem, 0, len(buildItems)),
	}
	//遍历商品， 逐个检查 + 组装
	for _, buildItem := range buildItems {
		product, err := productDB.GetProductById(buildItem.ProductID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, core.NewBizError(http.StatusNotFound, "商品不存在或已下架")
			}
			return nil, err
		}
		if product.Status != model.ProductOnSale {
			return nil, core.NewBizError(http.StatusBadRequest, "商品已下架")
		}
		if product.Stock < buildItem.Quantity {
			return nil, core.NewBizError(http.StatusBadRequest, "库存不足")
		}
		//计算商品总价 & 订单总价
		subtotal := product.PriceCents * int64(buildItem.Quantity)
		order.TotalAmount += subtotal
		order.Items = append(order.Items, model.OrderItem{
			ProductID:    product.ID,
			ProductName:  product.Name,
			ProductImage: firstProductImage(product),
			PriceCents:   product.PriceCents,
			Quantity:     buildItem.Quantity,
			Subtotal:     subtotal,
		})
	}

	return order, nil
}

// 取商品第一张图片
func firstProductImage(product *model.Product) string {
	if len(product.Images) == 0 {
		return ""
	}
	return product.Images[0].URL
}

func generateOrderNo(userID uint) string {
	return fmt.Sprintf("%d%06d", time.Now().UnixNano(), userID)
}

func (s *OrderService) CancelTimeoutOrder(userID uint, orderID uint) error {
	rows, err := s.orderDB.Cancel(userID, orderID)
	if err != nil {
		slog.Error("auto cancel timeout order failed", "order_id", orderID, "user_id", userID, "error", err)
		return err
	}
	if rows > 0 {
		slog.Info("auto cancelled timeout order", "order_id", orderID, "user_id", userID)
	}
	return nil
}

// 发延迟关单消息。
// 这是尽力而为的旁路动作：调用时订单已经落库，发消息失败绝不能让调用方以为「建单失败」——
// 那会触发调用方的库存回滚，而订单行还在，结果是孤儿订单 + 库存被重复卖出（超卖）。
// 所以失败只记日志，订单保持 pending_pay，靠对账兜底。
func (s *OrderService) publishOrderTimeout(order *model.Order) {
	if s.producer == nil {
		// 以前这里是静默 return，导致 worker 传 nil producer 时「秒杀订单永不超时取消」
		// 这个 bug 一直是隐形的。改成 Warn，让这类漏接线在日志里看得见。
		slog.Warn("order timeout producer not injected, skip delay message", "order_id", order.ID)
		return
	}
	msg := mq.Message{
		MsgID:     fmt.Sprintf("order-%d-%d", order.ID, time.Now().UnixNano()),
		Event:     "order.created", // 事件：订单已创建
		BizID:     order.ID,        // 订单ID
		UserID:    order.UserID,    // 用户ID
		CreatedAt: time.Now(),      // 发送时间
	}
	if err := s.producer.PublishDelayJSON(
		context.Background(),
		mq.OrderDelayRoutingKey,
		msg,            // 消息内容
		msg.MsgID,      // 唯一ID，防止重复
		30*time.Minute, // 延迟时间：30分钟
	); err != nil {
		slog.Error("publish order timeout message failed",
			"order_id", order.ID, "user_id", order.UserID, "error", err)
	}
}

// ListOrdersInput 用户订单列表查询
type ListOrdersInput struct {
	UserID   uint
	Status   string
	Page     int
	PageSize int
}

// ListOrdersResult 分页订单列表
type ListOrdersResult struct {
	Items    []*model.Order `json:"items"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

// ListOrders 用户查询自己的订单列表（按创建时间倒序，含商品快照）
func (s *OrderService) ListOrders(input ListOrdersInput) (*ListOrdersResult, error) {
	if input.Page <= 0 {
		input.Page = 1
	}
	if input.PageSize <= 0 {
		input.PageSize = 20
	}
	if input.PageSize > 100 {
		input.PageSize = 100
	}
	orders, total, err := s.orderDB.ListByUser(dao.OrderListQuery{
		UserID:   input.UserID,
		Status:   input.Status,
		Page:     input.Page,
		PageSize: input.PageSize,
	})
	if err != nil {
		return nil, err
	}
	return &ListOrdersResult{
		Items:    orders,
		Total:    total,
		Page:     input.Page,
		PageSize: input.PageSize,
	}, nil
}
