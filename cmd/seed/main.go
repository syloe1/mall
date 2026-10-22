package main

// 演示数据种子 / 每日重置工具。
//
// 用途：
//   - 保证演示账号可用：demo / Demo@123456（普通用户）、admin / Admin@123456（管理员）
//   - 分类字典入库（前端不再硬编码）
//   - 商品图切换为自托管 SVG（替换第三方 picsum.photos）
//   - 秒杀活动滚动窗口（now-1h ~ now+24h），并重新预热 Redis 库存
//   - --reset 时清空 demo 账号的购物车 / 订单 / 收藏 / 地址并按初始状态重建
//
// 用法：
//   ./mall-seed            # 幂等补齐（可安全重复执行）
//   ./mall-seed --reset    # 演示数据每日重置（服务器 cron 使用）
//
// 生产环境需要 AES_KEY（余额加密）与数据库配置，建议从 systemd 环境读取。

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"mall/model"
	"mall/pkg/aes"
	"mall/pkg/bootstrap"
	"mall/pkg/seckill"

	"github.com/go-redis/redis/v8"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	demoUsername  = "demo"
	demoPassword  = "Demo@123456"
	adminUsername = "admin"
	adminPassword = "Admin@123456"
	initialAmount = "10000.00"
	bcryptCost    = 12
)

type seed struct {
	db  *gorm.DB
	rdb *redis.Client
}

func main() {
	reset := flag.Bool("reset", false, "重置演示账号数据（每日 cron 使用）")
	flag.Parse()

	if os.Getenv("AES_KEY") == "" {
		log.Fatal("AES_KEY is required（余额字段用 AES 加密，生产环境由 systemd/环境变量提供）")
	}

	cfg, err := bootstrap.LoadWebConfig()
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}
	db, rdb, cleanup, err := bootstrap.InitStore(cfg)
	if err != nil {
		log.Fatalf("init store failed: %v", err)
	}
	defer cleanup()

	s := &seed{db: db, rdb: rdb}
	ctx := context.Background()

	if err := s.seedCategories(); err != nil {
		log.Fatalf("seed categories failed: %v", err)
	}
	demoUser, err := s.seedUsers()
	if err != nil {
		log.Fatalf("seed users failed: %v", err)
	}
	if *reset {
		if err := s.resetDemoData(demoUser); err != nil {
			log.Fatalf("reset demo data failed: %v", err)
		}
	}
	if err := s.seedDemoAddress(demoUser); err != nil {
		log.Fatalf("seed address failed: %v", err)
	}
	if err := s.selfHostProductImages(); err != nil {
		log.Fatalf("self-host product images failed: %v", err)
	}
	if err := s.rollSeckillActivity(ctx); err != nil {
		log.Fatalf("roll seckill activity failed: %v", err)
	}
	log.Printf("seed done: demo=%s admin=%s reset=%v", demoUsername, adminUsername, *reset)
}

func (s *seed) seedCategories() error {
	categories := []model.Category{
		{ID: 1, Name: "数码", SortOrder: 1},
		{ID: 2, Name: "食品", SortOrder: 2},
		{ID: 3, Name: "美妆", SortOrder: 3},
		{ID: 4, Name: "服饰", SortOrder: 4},
		{ID: 5, Name: "家电", SortOrder: 5},
		{ID: 6, Name: "家居", SortOrder: 6},
	}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "sort_order"}),
	}).Create(&categories).Error
}

func (s *seed) seedUsers() (*model.User, error) {
	demoUser, err := s.upsertUser(demoUsername, demoPassword, model.RoleUser, "演示用户")
	if err != nil {
		return nil, err
	}
	if _, err := s.upsertUser(adminUsername, adminPassword, model.RoleAdmin, "演示管理员"); err != nil {
		return nil, err
	}
	return demoUser, nil
}

func (s *seed) upsertUser(username, password, role, nickname string) (*model.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, err
	}
	amount, err := aes.Encrypt(initialAmount)
	if err != nil {
		return nil, err
	}

	var user model.User
	findErr := s.db.Where("username = ?", username).First(&user).Error
	if findErr == nil {
		updates := map[string]interface{}{
			"password": string(hash),
			"role":     role,
			"nickname": nickname,
			"amount":   amount,
			"avatar":   model.DefaultAvator,
		}
		if err := s.db.Model(&user).Updates(updates).Error; err != nil {
			return nil, err
		}
		user.Password = string(hash)
		user.Role = role
		return &user, nil
	}
	if findErr != gorm.ErrRecordNotFound {
		return nil, findErr
	}
	user = model.User{
		Username: username,
		Nickname: nickname,
		Avatar:   model.DefaultAvator,
		Password: string(hash),
		Role:     role,
		Amount:   amount,
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// resetDemoData 每天把演示账号恢复到初始状态：购物车/订单/收藏/地址清空重建。
func (s *seed) resetDemoData(user *model.User) error {
	tx := s.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	cleanup := func() {
		if err := tx.Exec("DELETE FROM order_items WHERE order_id IN (SELECT id FROM orders WHERE user_id = ?)", user.ID).Error; err != nil {
			log.Printf("warn: delete order items: %v", err)
		}
		for _, stmt := range []string{
			"DELETE FROM payments WHERE user_id = ?",
			"DELETE FROM orders WHERE user_id = ?",
			"DELETE FROM cart_items WHERE user_id = ?",
			"DELETE FROM favorite_items WHERE user_id = ?",
			"DELETE FROM favorite_folders WHERE user_id = ?",
			"DELETE FROM user_addresses WHERE user_id = ?",
			"DELETE FROM seckill_purchases WHERE user_id = ?",
		} {
			if err := tx.Exec(stmt, user.ID).Error; err != nil {
				log.Printf("warn: reset demo data (%s): %v", stmt, err)
			}
		}
	}
	cleanup()
	if err := tx.Commit().Error; err != nil {
		return err
	}
	// 清掉 demo 账号的 Redis 秒杀购买记录与登录态
	ctx := context.Background()
	keys, err := s.rdb.Keys(ctx, fmt.Sprintf("seckill:buy:*:%d", user.ID)).Result()
	if err == nil && len(keys) > 0 {
		_ = s.rdb.Del(ctx, keys...).Err()
	}
	_ = s.rdb.Del(ctx, fmt.Sprintf("auth:refresh:user:%d", user.ID)).Err()
	return nil
}

func (s *seed) seedDemoAddress(user *model.User) error {
	var address model.UserAddress
	err := s.db.Where("user_id = ?", user.ID).Order("id ASC").First(&address).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return s.db.Create(&model.UserAddress{
		UserID:       user.ID,
		ReceiverName: "演示用户",
		Phone:        "13800138000",
		Province:     "安徽省",
		City:         "合肥市",
		District:     "蜀山区",
		Detail:       "演示地址 · 科学大道 100 号",
		PostalCode:   "230000",
		IsDefault:    true,
	}).Error
}

// selfHostProductImages 把商品图从 picsum.photos 换成本地 SVG 占位图
// （前端 public/img/products/p{product_id}.svg 由 mall-Web 构建产物托管）。
func (s *seed) selfHostProductImages() error {
	type imageRow struct {
		ProductID uint
	}
	var rows []imageRow
	if err := s.db.Model(&model.ProductImage{}).
		Where("url LIKE ?", "%picsum.photos%").
		Group("product_id").
		Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	if err := s.db.Model(&model.ProductImage{}).
		Where("url LIKE ?", "%picsum.photos%").
		Update("url", gorm.Expr("CONCAT('/img/products/p', product_id, '.svg')")).Error; err != nil {
		return err
	}
	if err := s.db.Model(&model.OrderItem{}).
		Where("product_image LIKE ?", "%picsum.photos%").
		Update("product_image", gorm.Expr("CONCAT('/img/products/p', product_id, '.svg')")).Error; err != nil {
		log.Printf("warn: update order item images: %v", err)
	}
	// 触发搜索索引同步（worker 轮询 outbox）
	now := time.Now()
	events := make([]model.ProductOutbox, 0, len(rows))
	for _, row := range rows {
		events = append(events, model.ProductOutbox{
			EventID:   fmt.Sprintf("seed-image-%d-%d", row.ProductID, now.UnixNano()),
			EventType: model.ProductEventUpdated,
			ProductID: row.ProductID,
			Status:    model.ProductOutboxPending,
		})
	}
	if err := s.db.Create(&events).Error; err != nil {
		return err
	}
	log.Printf("self-hosted product images: %d products", len(rows))
	return nil
}

// rollSeckillActivity 把秒杀活动滚动到 now-1h ~ now+24h，并重新预热 Redis 库存。
func (s *seed) rollSeckillActivity(ctx context.Context) error {
	now := time.Now()
	start := now.Add(-time.Hour)
	end := now.Add(24 * time.Hour)

	// 先清理历史遗留的同名演示活动（含软删记录），保证 activity_no 唯一索引不冲突
	if err := s.db.Unscoped().
		Where("activity_no = ? AND id <> ?", "SK-DEMO-001", 1).
		Delete(&model.SeckillActivity{}).Error; err != nil {
		return err
	}
	var activity model.SeckillActivity
	err := s.db.Unscoped().Where("id = ?", 1).First(&activity).Error
	newFields := map[string]interface{}{
		"activity_no":          "SK-DEMO-001",
		"name":                 "演示秒杀 · 无线降噪耳机 Pro",
		"product_id":           uint(1001),
		"product_name":         "无线降噪耳机 Pro",
		"product_image":        "/img/products/p1001.svg",
		"original_price_cents": int64(89900),
		"seckill_price_cents":  int64(49900),
		"total_stock":          50,
		"available_stock":      50,
		"per_limit":            1,
		"start_at":             start,
		"end_at":               end,
		"status":               model.SeckillActivityRunning,
	}
	switch {
	case err == nil:
		if err := s.db.Model(&activity).Updates(newFields).Error; err != nil {
			return err
		}
	case err == gorm.ErrRecordNotFound:
		// 活动 ID 固定为 1：秒杀 Redis 库存 key、前端演示入口都按 1 约定
		if err := s.db.Unscoped().Delete(&model.SeckillActivity{}, 1).Error; err != nil {
			return err
		}
		activity = model.SeckillActivity{StartAt: start, EndAt: end}
		activity.ID = 1
		if err := s.db.Create(&activity).Error; err != nil {
			return err
		}
		if err := s.db.Model(&activity).Updates(newFields).Error; err != nil {
			return err
		}
	default:
		return err
	}

	// 重新预热 Redis 库存：先删旧 key，再用剩余 TTL 写入
	_ = s.rdb.Del(ctx, seckill.StockKey(1)).Err()
	ttl := time.Until(end) + time.Hour
	if ttl > 0 {
		if err := seckill.InitStock(ctx, s.rdb, 1, 50, ttl); err != nil {
			return err
		}
	}
	log.Printf("seckill activity 1 rolled: %s ~ %s", start.Format(time.RFC3339), end.Format(time.RFC3339))
	return nil
}
