package dto

import "time"

// 秒杀活动（公开列表）。status 是服务端按 start_at/end_at 实时派生的
// upcoming / running / ended，前端不再把数据库 status 当唯一事实来源。
type SeckillActivityResponse struct {
	ID                 uint      `json:"id"`
	ActivityNo         string    `json:"activity_no"`
	Name               string    `json:"name"`
	ProductID          uint      `json:"product_id"`
	ProductName        string    `json:"product_name"`
	ProductImage       string    `json:"product_image"`
	OriginalPriceCents int64     `json:"original_price_cents"`
	SeckillPriceCents  int64     `json:"seckill_price_cents"`
	TotalStock         int       `json:"total_stock"`
	AvailableStock     int       `json:"available_stock"`
	PerLimit           int       `json:"per_limit"`
	StartAt            time.Time `json:"start_at"`
	EndAt              time.Time `json:"end_at"`
	Status             string    `json:"status"`
}

type SeckillInitStockReq struct {
	ActivityID uint `json:"activity_id" binding:"required"`
}
