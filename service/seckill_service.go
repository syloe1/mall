package service

import (
	"context"
	"errors"
	"fmt"
	"mall/dao"
	"mall/model"
	"mall/pkg/core"
	"mall/pkg/lock"
	"mall/pkg/mq"
	"mall/pkg/seckill"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// SeckillOrderCreator 由 OrderService 实现：消费者据此把秒杀单落成真实订单
type SeckillOrderCreator interface {
	CreateSeckillOrder(userID uint, activity *model.SeckillActivity) (*model.Order, error)
}

type SeckillService struct {
	rdb          *redis.Client
	seckillDB    dao.SeckillDB
	producer     *mq.Producer
	orderCreator SeckillOrderCreator // 可选：注入后消费者会创建真实订单
}

func NewSeckillService(rdb *redis.Client, seckillDB dao.SeckillDB, producer *mq.Producer) *SeckillService {
	return &SeckillService{
		rdb:       rdb,
		seckillDB: seckillDB,
		producer:  producer,
	}
}

// SetOrderCreator 注入订单创建器（worker 侧调用；API 侧不需要）
func (s *SeckillService) SetOrderCreator(c SeckillOrderCreator) {
	s.orderCreator = c
}

// ListActivities 返回当前进行中 / 未开始 / 最近结束的秒杀活动。
// Database 里的 status 不再直接下发给前端，统一用时间窗口派生。
func (s *SeckillService) ListActivities() ([]*model.SeckillActivity, error) {
	activities, err := s.seckillDB.ListActiveActivities()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	for _, activity := range activities {
		activity.Status = activity.DerivedStatus(now)
	}
	return activities, nil
}

func (s *SeckillService) Buy(activityID, userID uint) error {
	if activityID == 0 {
		return core.NewBizError(http.StatusBadRequest, "秒杀活动 ID 不合法")
	}
	if userID == 0 {
		return core.NewBizError(http.StatusBadRequest, "用户 ID 不合法")
	}

	ctx := context.Background()
	//防止同一个用户重复提交（疯狂点抢购按钮）
	lockKey := fmt.Sprintf("lock:seckill:%d:%d", activityID, userID)
	// 锁 3 秒：防止用户重复请求
	l := lock.NewRedisLock(s.rdb, lockKey, 3*time.Second)
	if err := l.Lock(ctx); err != nil {
		if errors.Is(err, lock.ErrLockNotAcquired) {
			return core.NewBizError(http.StatusTooManyRequests, "秒杀请求处理中，请稍后再试")
		}
		return err
	}
	defer func() { _ = l.Unlock(ctx) }()
	// 加载秒杀活动（校验是否有效、时间是否正确）
	activity, err := s.loadActiveActivity(activityID)
	if err != nil {
		return err
	}
	//返回 3 种状态：1=成功 0=售完 -1=已买过
	code, err := seckill.TryDeduct(ctx, s.rdb, activityID, userID, 10*time.Minute)
	if err != nil {
		return err
	}

	switch code {
	case 1:
		//发送消息到 MQ，异步创建订单
		if err := s.publishSeckillOrder(activity, userID); err != nil {
			//失败回滚
			_ = seckill.RollbackDeduct(ctx, s.rdb, activityID, userID)
			return err
		}
		return nil
	case 0:
		return core.NewBizError(http.StatusConflict, "商品已抢光")
	case -1:
		return core.NewBizError(http.StatusConflict, "你已参与过该活动，请勿重复抢购")
	default:
		return core.NewBizError(http.StatusInternalServerError, core.MsgInternal)
	}
}

func (s *SeckillService) CreatePurchaseFromMessage(activityID uint, userID uint) error {
	if activityID == 0 || userID == 0 {
		return core.NewBizError(http.StatusBadRequest, "秒杀消息参数不合法")
	}
	//从 MySQL 查询秒杀活动（确保活动存在）
	activity, err := s.seckillDB.GetActivityByID(activityID)
	if err != nil {
		return err
	}

	// 未注入订单创建器（单测/降级路径）：保持只写购买记录的旧行为
	if s.orderCreator == nil {
		if err := s.createSeckillPurchase(activity, userID); err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return nil
			}
			_ = seckill.RollbackDeduct(context.Background(), s.rdb, activityID, userID)
			return err
		}
		return nil
	}

	// 正式链路：先建真实待支付订单，再写购买记录（记录里带上 order_id）
	order, err := s.orderCreator.CreateSeckillOrder(userID, activity)
	if err != nil {
		// 建单失败（如无地址）→ 回滚 Redis 预扣库存
		_ = seckill.RollbackDeduct(context.Background(), s.rdb, activityID, userID)
		return err
	}
	purchase := &model.SeckillPurchase{
		UserID:            userID,
		SeckillActivityID: activity.ID,
		ProductID:         activity.ProductID,
		OrderID:           order.ID,
		Status:            model.SeckillPurchaseCreated,
	}
	if err := s.seckillDB.CreatePurchase(purchase); err != nil {
		// MQ 重复投递 → 已处理过，幂等返回
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil
		}
		_ = seckill.RollbackDeduct(context.Background(), s.rdb, activityID, userID)
		return err
	}
	return nil
}

// MyPurchase 查询当前用户在某活动的抢购结果（前端轮询）
// 未抢到（记录不存在）返回 nil,nil
func (s *SeckillService) MyPurchase(userID, activityID uint) (*model.SeckillPurchase, error) {
	p, err := s.seckillDB.GetPurchase(userID, activityID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

func (s *SeckillService) InitActivityStock(activityID uint) error {
	if activityID == 0 {
		return core.NewBizError(http.StatusBadRequest, "秒杀活动 ID 不合法")
	}

	activity, err := s.seckillDB.GetActivityByID(activityID)
	if err != nil {
		return err
	}
	if status := activity.DerivedStatus(time.Now()); status == model.SeckillActivityClosed || status == model.SeckillActivityEnded || status == model.SeckillActivityDraft {
		return core.NewBizError(http.StatusBadRequest, "活动已结束或未开放，无法初始化库存")
	}
	//加一个一小时对账
	ttl := time.Until(activity.EndAt) + time.Hour
	if ttl <= 0 {
		return core.NewBizError(http.StatusBadRequest, "活动已结束")
	}
	//把库存写入 Redis（预热）
	if err := seckill.InitStock(context.Background(), s.rdb, activity.ID, activity.AvailableStock, ttl); err != nil {
		if errors.Is(err, seckill.ErrStockAlreadyInitialized) {
			return core.NewBizError(http.StatusBadRequest, "秒杀库存已初始化，无需重复预热")
		}
		return err
	}

	return nil
}

// loadActiveActivity 只允许“时间窗口内 + 未被人工关闭”的活动抢购。
// ended 返回 410 Gone、未开始返回 409 Conflict，前端可据此给出准确提示。
func (s *SeckillService) loadActiveActivity(activityID uint) (*model.SeckillActivity, error) {
	activity, err := s.seckillDB.GetActivityByID(activityID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.NewBizError(http.StatusNotFound, "秒杀活动不存在")
		}
		return nil, err
	}
	switch activity.DerivedStatus(time.Now()) {
	case model.SeckillActivityClosed:
		return nil, core.NewBizError(http.StatusGone, "秒杀活动已关闭")
	case model.SeckillActivityEnded:
		return nil, core.NewBizError(http.StatusGone, "秒杀活动已结束")
	case model.SeckillActivityUpcoming:
		return nil, core.NewBizError(http.StatusConflict, "秒杀活动尚未开始")
	case model.SeckillActivityDraft:
		return nil, core.NewBizError(http.StatusGone, "秒杀活动未开放")
	}
	return activity, nil
}

// 创建秒杀购买记录（落库）
func (s *SeckillService) createSeckillPurchase(activity *model.SeckillActivity, userID uint) error {
	// 构造一条秒杀购买记录
	purchase := &model.SeckillPurchase{
		UserID:            userID,                       // 谁抢到了
		SeckillActivityID: activity.ID,                  // 哪个秒杀活动
		ProductID:         activity.ProductID,           // 哪个商品
		OrderID:           0,                            // 订单ID还没生成（后面生成）
		Status:            model.SeckillPurchaseCreated, // 状态：已创建
	}

	// 写入 MySQL 数据库
	return s.seckillDB.CreatePurchase(purchase)
}

// 秒杀成功 → 发送 MQ 消息，异步创建订单
func (s *SeckillService) publishSeckillOrder(activity *model.SeckillActivity, userID uint) error {
	// 如果没有 MQ（降级），直接同步创建订单（不推荐高并发用）
	if s.producer == nil {
		return s.createSeckillPurchase(activity, userID)
	}

	// 构造 MQ 消息（唯一、可追踪）
	msg := mq.Message{
		MsgID:     fmt.Sprintf("seckill-%d-%d-%d", activity.ID, userID, time.Now().UnixNano()), // 唯一消息ID
		Event:     "seckill.order.create",                                                      // 事件类型：创建秒杀订单
		BizID:     activity.ID,                                                                 // 业务ID：秒杀活动ID
		UserID:    userID,                                                                      // 购买用户
		CreatedAt: time.Now(),                                                                  // 时间
		Payload: map[string]any{ // 消息内容
			"activity_id": activity.ID,
			"product_id":  activity.ProductID,
		},
	}

	// 🔥 发送消息到 RabbitMQ
	return s.producer.PublishJSON(context.Background(), mq.SeckillOrderKey, msg, msg.MsgID)
}
