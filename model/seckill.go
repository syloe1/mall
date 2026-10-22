package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	SeckillActivityDraft    = "draft"
	SeckillActivityUpcoming = "upcoming"
	SeckillActivityRunning  = "running"
	SeckillActivityEnded    = "ended"  //正常结束
	SeckillActivityClosed   = "closed" //手动关闭
)

const (
	SeckillPurchaseCreated  = "created"
	SeckillPurchasePaid     = "paid"
	SeckillPurchaseCanceled = "canceled"
	SeckillPurchaseFailed   = "failed"
)

type SeckillActivity struct {
	gorm.Model

	ActivityNo string `gorm:"size:64;uniqueIndex;not null" json:"activity_no"`

	Name string `gorm:"size:128;not null" json:"name"`

	ProductID    uint   `gorm:"index;not null" json:"product_id"`
	ProductName  string `gorm:"size:128;not null" json:"product_name"`
	ProductImage string `gorm:"size:255" json:"product_image"`

	OriginalPriceCents int64 `gorm:"not null" json:"original_price_cents"`
	SeckillPriceCents  int64 `gorm:"not null" json:"seckill_price_cents"`

	TotalStock     int `gorm:"not null" json:"total_stock"`
	AvailableStock int `gorm:"not null" json:"available_stock"`
	PerLimit       int `gorm:"not null;default:1" json:"per_limit"` //限购数量

	StartAt time.Time `gorm:"index;not null" json:"start_at"`
	EndAt   time.Time `gorm:"index;not null" json:"end_at"`

	Status string `gorm:"size:32;index;not null" json:"status"`
}

// DerivedStatus 用 start_at / end_at 实时派生活动状态。
//
// 审计问题：数据库里的 status 字段可能因没有定时任务而停留在 running，
// 导致已过期活动在前端仍显示“进行中”。状态的唯一事实来源应该是时间窗口，
// status 只作为人工关闭（closed）/草稿（draft）的覆盖标记。
func (a *SeckillActivity) DerivedStatus(now time.Time) string {
	switch a.Status {
	case SeckillActivityDraft:
		return SeckillActivityDraft
	case SeckillActivityClosed:
		return SeckillActivityClosed
	case SeckillActivityEnded:
		return SeckillActivityEnded
	}
	if now.Before(a.StartAt) {
		return SeckillActivityUpcoming
	}
	if !now.Before(a.EndAt) {
		return SeckillActivityEnded
	}
	return SeckillActivityRunning
}

type SeckillPurchase struct {
	gorm.Model

	UserID            uint   `gorm:"uniqueIndex:uk_user_activity;index;not null" json:"user_id"`
	SeckillActivityID uint   `gorm:"uniqueIndex:uk_user_activity;index;not null" json:"seckill_activity_id"`
	ProductID         uint   `gorm:"index;not null" json:"product_id"`
	OrderID           uint   `gorm:"index;not null" json:"order_id"`
	Status            string `gorm:"size:32;index;not null" json:"status"`
}
