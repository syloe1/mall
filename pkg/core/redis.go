package core

import (
	"fmt"
	"mall/config"

	redisotel "github.com/go-redis/redis/extra/redisotel/v8"
	"github.com/go-redis/redis/v8"
)

// 创建并返回一个 Redis 客户端
func InitRedis(cfg config.RedisConfig) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	rdb.AddHook(redisotel.NewTracingHook())
	return rdb
}
