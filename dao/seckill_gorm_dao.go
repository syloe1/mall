package dao

import (
	"errors"
	"mall/model"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type GormSeckillDB struct {
	DB *gorm.DB
}

func (g *GormSeckillDB) GetActivityByID(activityID uint) (*model.SeckillActivity, error) {
	var activity model.SeckillActivity
	if err := g.DB.First(&activity, activityID).Error; err != nil {
		return nil, err
	}
	return &activity, nil
}

// ListActiveActivities 查询进行中 / 未开始 / 最近 7 天内结束的活动，按开始时间倒序。
//
// 之所以把“最近结束”的活动也带出来：数据库 status 可能是历史遗留的 running，
// 由服务层用 start_at/end_at 实时派生真实状态，前端据此显示「已结束」并禁用按钮。
func (g *GormSeckillDB) ListActiveActivities() ([]*model.SeckillActivity, error) {
	var activities []*model.SeckillActivity
	err := g.DB.
		Where("status IN ?", []string{model.SeckillActivityRunning, model.SeckillActivityUpcoming}).
		Where("end_at > ?", time.Now().Add(-7*24*time.Hour)).
		Order("start_at DESC").
		Find(&activities).Error
	if err != nil {
		return nil, err
	}
	return activities, nil
}

// GetPurchase 查询某用户在某活动的购买记录（抢购结果轮询用）
func (g *GormSeckillDB) GetPurchase(userID, activityID uint) (*model.SeckillPurchase, error) {
	var purchase model.SeckillPurchase
	err := g.DB.
		Where("user_id = ? AND seckill_activity_id = ?", userID, activityID).
		First(&purchase).Error
	if err != nil {
		return nil, err
	}
	return &purchase, nil
}

func (g *GormSeckillDB) CreatePurchase(purchase *model.SeckillPurchase) error {
	err := g.DB.Create(purchase).Error
	if isDuplicatedKey(err) {
		return gorm.ErrDuplicatedKey
	}
	return err
}

func isDuplicatedKey(err error) bool {
	if err == nil {
		return false
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate")
}
