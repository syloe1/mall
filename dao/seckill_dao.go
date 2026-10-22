package dao

import "mall/model"

type SeckillDB interface {
	GetActivityByID(activityID uint) (*model.SeckillActivity, error)
	// ListActiveActivities 查询进行中 / 未开始的活动（按开始时间倒序），供前台列表展示
	ListActiveActivities() ([]*model.SeckillActivity, error)
	// GetPurchase 查询某用户在某活动的购买记录（抢购结果轮询用）
	GetPurchase(userID, activityID uint) (*model.SeckillPurchase, error)
	//创建秒杀购买记录
	CreatePurchase(purchase *model.SeckillPurchase) error
}
