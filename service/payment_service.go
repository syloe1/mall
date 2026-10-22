package service

import (
	"context"
	"errors"
	"fmt"
	"mall/dao"
	"mall/model"
	"mall/pkg/aes"
	"mall/pkg/core"
	"mall/pkg/lock"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type PaymentService struct {
	paymentDB dao.PaymentDB
	orderDB   dao.OrderDB
	userDB    dao.UserDB
	rdb       *redis.Client //防重复支付
}

func NewPaymentService(paymentDB dao.PaymentDB, orderDB dao.OrderDB, userDB dao.UserDB, rdb *redis.Client) *PaymentService {
	return &PaymentService{
		paymentDB: paymentDB,
		orderDB:   orderDB,
		userDB:    userDB,
		rdb:       rdb,
	}
}

type CreatePaymentInput struct {
	UserID      uint   // 谁支付
	OrderID     uint   // 支付哪个订单
	Channel     string // 支付渠道：alipay / wechat / balance
	PayPassword string // 支付密码（余额支付时用）
}

type GetPaymentInput struct {
	UserID    uint   // 谁查询
	PaymentNo string // 支付单号
}

type MockPaySuccessInput struct {
	UserID    uint
	PaymentNo string
}

func (s *PaymentService) CreatePayment(input CreatePaymentInput) (*model.Payment, error) {
	if input.UserID == 0 {
		return nil, core.NewBizError(http.StatusBadRequest, "用户 ID 不合法")
	}
	if input.OrderID == 0 {
		return nil, core.NewBizError(http.StatusBadRequest, "订单 ID 不合法")
	}

	channel := strings.TrimSpace(input.Channel)
	if channel == "" {
		return nil, core.NewBizError(http.StatusBadRequest, "支付渠道不合法")
	}
	if channel != model.PaymentChannelMock && channel != model.PaymentChannelBalance {
		return nil, core.NewBizError(http.StatusBadRequest, "暂不支持该支付渠道")
	}
	//加分布式锁， 防止重复创建支付单
	return s.withPaymentLock(fmt.Sprintf("lock:payment:order:%d", input.OrderID), func() (*model.Payment, error) {
		order, err := s.orderDB.GetByID(input.UserID, input.OrderID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, core.NewBizError(http.StatusNotFound, "订单不存在")
			}
			return nil, err
		}
		// 已经支付过
		if order.Status == model.OrderPaid {
			return nil, core.NewBizError(http.StatusBadRequest, "订单已支付")
		}
		// 不是待支付状态，不能支付
		if order.Status != model.OrderPendingPay {
			return nil, core.NewBizError(http.StatusBadRequest, "订单当前状态不可支付")
		}
		if order.TotalAmount <= 0 {
			return nil, core.NewBizError(http.StatusBadRequest, "订单金额不合法")
		}

		payment, err := s.paymentDB.GetSuccessByOrderID(input.UserID, input.OrderID)
		// 已经支付成功 → 直接返回支付单
		if err == nil {
			return payment, nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		//查询是否已经存在待支付的支付单
		payment, err = s.paymentDB.GetByOrderID(input.UserID, input.OrderID)
		if err == nil {
			// 如果已有待支付支付单
			if payment.Status == model.PaymentStatusPending {
				if channel == model.PaymentChannelBalance {
					return s.payByBalanceLocked(input.UserID, payment.OrderID, input.PayPassword)
				}
				return payment, nil
			}
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		// 2. 没有支付单 → 如果是余额支付，直接走余额创建+扣款
		if channel == model.PaymentChannelBalance {
			return s.payByBalanceLocked(input.UserID, order.ID, input.PayPassword)
		}

		payment = &model.Payment{
			PaymentNo: generatePaymentNo(input.UserID),
			OrderID:   order.ID,
			OrderNo:   order.OrderNo,
			UserID:    input.UserID,
			Channel:   channel,
			Amount:    order.TotalAmount,
			Status:    model.PaymentStatusPending,
		}

		if err := s.paymentDB.Create(payment); err != nil {
			return nil, err
		}

		return payment, nil
	})
}

func (s *PaymentService) payByBalanceLocked(userID uint, orderID uint, payPassword string) (*model.Payment, error) {
	payPassword = strings.TrimSpace(payPassword)
	if payPassword == "" {
		return nil, core.NewBizError(http.StatusBadRequest, "请输入支付密码")
	}

	var paidPayment *model.Payment
	err := s.userDB.Transaction(func(userTx dao.UserDB) error {
		orderTx := s.orderDB
		if gormUserTx, ok := userTx.(*dao.GormUserDB); ok {
			orderTx = &dao.GormOrderDB{DB: gormUserTx.DB}
		}
		paymentTx := s.paymentDB
		if gormUserTx, ok := userTx.(*dao.GormUserDB); ok {
			paymentTx = &dao.GormPaymentDB{DB: gormUserTx.DB}
		}
		//同一个用户不能同时发起两笔支付！
		user, err := userTx.FindByIDForUpdate(userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return core.NewBizError(http.StatusNotFound, "用户不存在")
			}
			return err
		}
		//校验密码
		if strings.TrimSpace(user.PayPassword) == "" {
			return core.NewBizError(http.StatusBadRequest, "尚未设置支付密码")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PayPassword), []byte(payPassword)); err != nil {
			return core.NewBizError(http.StatusBadRequest, "支付密码错误")
		}
		//查询订单枷锁
		order, err := orderTx.GetByIDForUpdate(userID, orderID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return core.NewBizError(http.StatusNotFound, "订单不存在")
			}
			return err
		}
		if order.Status == model.OrderPaid {
			//已支付 -> 直接返回支付单
			payment, err := paymentTx.GetByOrderIDForUpdate(userID, orderID)
			if err != nil {
				return err
			}
			paidPayment = payment
			return nil
		}
		// 不是待支付 → 不能付款
		if order.Status != model.OrderPendingPay {
			return core.NewBizError(http.StatusBadRequest, "订单当前状态不可支付")
		}
		//获取 / 创建支付单
		payment, err := paymentTx.GetByOrderIDForUpdate(userID, orderID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				payment = &model.Payment{
					PaymentNo: generatePaymentNo(userID),
					OrderID:   order.ID,
					OrderNo:   order.OrderNo,
					UserID:    userID,
					Channel:   model.PaymentChannelBalance,
					Amount:    order.TotalAmount,
					Status:    model.PaymentStatusPending,
				}
				if err := paymentTx.Create(payment); err != nil {
					return err
				}
			} else {
				return err
			}
		}
		if payment.Channel != model.PaymentChannelBalance {
			return core.NewBizError(http.StatusBadRequest, "支付渠道不匹配")
		}
		if payment.Status == model.PaymentStatusSuccess {
			paidPayment = payment
			return nil
		}
		if payment.Status != model.PaymentStatusPending {
			return core.NewBizError(http.StatusBadRequest, "支付单当前状态不可支付")
		}
		if payment.Amount != order.TotalAmount {
			return core.NewBizError(http.StatusBadRequest, "支付金额不一致")
		}
		//解密余额，判断是否足够
		balanceCents, err := decryptAmountCents(user.Amount)
		if err != nil {
			return core.NewBizError(http.StatusInternalServerError, "余额解密失败")
		}
		if balanceCents < payment.Amount {
			return core.NewBizError(http.StatusBadRequest, "余额不足")
		}
		//扣减余额
		user.Amount, err = encryptAmountCents(balanceCents - payment.Amount)
		if err != nil {
			return core.NewBizError(http.StatusInternalServerError, "余额加密失败")
		}
		if err := userTx.Update(user); err != nil {
			return err
		}

		now := time.Now()
		payment.Status = model.PaymentStatusSuccess
		payment.PaidAt = &now
		payment.TransactionNo = generateBalanceTransactionNo(userID)
		if err := paymentTx.Update(payment); err != nil {
			return err
		}
		if err := orderTx.UpdateStatus(order.ID, model.OrderPaid); err != nil {
			return err
		}

		paidPayment = payment
		return nil
	})
	if err != nil {
		return nil, err
	}
	return paidPayment, nil
}

// 给支付操作加一把 Redis 分布式锁，防止用户同时发起多次支付 → 避免重复扣款！
func (s *PaymentService) withPaymentLock(key string, fn func() (*model.Payment, error)) (*model.Payment, error) {
	if s.rdb == nil {
		return fn()
	}

	ctx := context.Background()
	l := lock.NewRedisLock(s.rdb, key, 5*time.Second)
	//尝试加锁
	if err := l.Lock(ctx); err != nil {
		if errors.Is(err, lock.ErrLockNotAcquired) {
			return nil, core.NewBizError(http.StatusTooManyRequests, "支付处理中，请稍后再试")
		}
		return nil, err
	}
	defer func() { _ = l.Unlock(ctx) }()

	return fn()
}

func (s *PaymentService) GetPayment(input GetPaymentInput) (*model.Payment, error) {
	if input.UserID == 0 {
		return nil, core.NewBizError(http.StatusBadRequest, "用户 ID 不合法")
	}

	paymentNo := strings.TrimSpace(input.PaymentNo)
	if paymentNo == "" {
		return nil, core.NewBizError(http.StatusBadRequest, "支付单号不合法")
	}

	payment, err := s.paymentDB.GetByPaymentNo(input.UserID, paymentNo)
	if err != nil {
		//没查到 404
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.NewBizError(http.StatusNotFound, "支付单不存在")
		}
		return nil, err
	}

	return payment, nil
}
func (s *PaymentService) MockPaySuccess(input MockPaySuccessInput) (*model.Payment, error) {
	payment, err := s.GetPayment(GetPaymentInput{
		UserID:    input.UserID,
		PaymentNo: input.PaymentNo,
	})
	if err != nil {
		return nil, err
	}

	return s.withPaymentLock(fmt.Sprintf("lock:payment:order:%d", payment.OrderID), func() (*model.Payment, error) {
		return s.mockPaySuccessLocked(input.UserID, payment)
	})
}

func (s *PaymentService) mockPaySuccessLocked(userID uint, payment *model.Payment) (*model.Payment, error) {
	if payment.Status == model.PaymentStatusSuccess {
		return payment, nil
	}
	if payment.Status != model.PaymentStatusPending {
		return nil, core.NewBizError(http.StatusBadRequest, "支付单当前状态不可支付")
	}

	order, err := s.orderDB.GetByID(userID, payment.OrderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.NewBizError(http.StatusNotFound, "订单不存在")
		}
		return nil, err
	}

	if order.Status == model.OrderPaid {
		// 只更新支付单为成功
		payment.Status = model.PaymentStatusSuccess
		now := time.Now()
		payment.PaidAt = &now
		payment.TransactionNo = generateMockTransactionNo(userID)
		if err := s.paymentDB.Update(payment); err != nil {
			return nil, err
		}
		return payment, nil
	}

	if order.Status != model.OrderPendingPay {
		return nil, core.NewBizError(http.StatusBadRequest, "订单当前状态不可支付")
	}
	if payment.Amount != order.TotalAmount {
		return nil, core.NewBizError(http.StatusBadRequest, "支付金额不一致")
	}

	now := time.Now()
	payment.Status = model.PaymentStatusSuccess
	payment.PaidAt = &now
	payment.TransactionNo = generateMockTransactionNo(userID)

	if err := s.paymentDB.Update(payment); err != nil {
		return nil, err
	}
	if err := s.orderDB.UpdateStatus(order.ID, model.OrderPaid); err != nil {
		return nil, err
	}

	return payment, nil
}
func generatePaymentNo(userID uint) string {
	return fmt.Sprintf("PAY%d%06d", time.Now().UnixNano(), userID)
}

func generateMockTransactionNo(userID uint) string {
	return fmt.Sprintf("MOCK%d%06d", time.Now().UnixNano(), userID)
}

func generateBalanceTransactionNo(userID uint) string {
	return fmt.Sprintf("BAL%d%06d", time.Now().UnixNano(), userID)
}

func decryptAmountCents(encryptedAmount string) (int64, error) {
	plainAmount, err := aes.Decrypt(encryptedAmount)
	if err != nil {
		return 0, err
	}
	return amountStringToCents(plainAmount)
}

func encryptAmountCents(amountCents int64) (string, error) {
	return aes.Encrypt(centsToAmountString(amountCents))
}

func amountStringToCents(amount string) (int64, error) {
	amount = strings.TrimSpace(amount)
	if amount == "" {
		return 0, nil
	}

	parts := strings.Split(amount, ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("invalid amount: %s", amount)
	}

	yuan, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, err
	}

	var cents int64
	if len(parts) == 2 {
		fraction := parts[1]
		if len(fraction) > 2 {
			return 0, fmt.Errorf("invalid amount precision: %s", amount)
		}
		for len(fraction) < 2 {
			fraction += "0"
		}
		cents, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, err
		}
	}

	return yuan*100 + cents, nil
}

func centsToAmountString(amountCents int64) string {
	return fmt.Sprintf("%d.%02d", amountCents/100, amountCents%100)
}
