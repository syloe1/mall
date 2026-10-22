package bootstrap

import (
	"context"
	"fmt"
	"log"
	"mall/config"
	"mall/database"
	"mall/pkg/core"
	"mall/pkg/tracex"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// 找配置文件
func ResolveConfigPath() (string, error) {
	return ResolveConfigPathFromCandidates([]string{
		"config/config.yaml",
		"../config/config.yaml",
	})
}

func ResolveConfigPathFromCandidates(candidates []string) (string, error) {
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("config file not found, checked: %v", candidates)
}

func LoadConfigFromCandidates(candidates []string) (*config.App, error) {
	configPath, err := ResolveConfigPathFromCandidates(candidates)
	if err != nil {
		return nil, err
	}
	return config.Init(configPath)
}

// 加载 API 服务配置
func LoadWebConfig() (*config.App, error) {
	return LoadConfigFromCandidates([]string{
		"config/config.yaml",
		"../config/config.yaml",
	})
}

// 加载后台任务（消费者）配置s
func LoadWorkerConfig() (*config.App, error) {
	return LoadConfigFromCandidates([]string{
		"config/worker.yaml",
		"../config/worker.yaml",
	})
}

func InitTrace(cfg *config.App) (func(context.Context) error, error) {
	return tracex.Init(context.Background(), cfg.GetTraceConfig())
}

func InitStore(cfg *config.App) (*gorm.DB, *redis.Client, func(), error) {
	db, err := core.InitMysql(cfg.GetMysqlConfig())
	if err != nil {
		return nil, nil, nil, err
	}
	rdb := core.InitRedis(cfg.GetRedisConfig())

	if err := database.AutoMigrate(db); err != nil {
		_ = rdb.Close()
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
		return nil, nil, nil, err
	}

	cleanup := func() {
		_ = rdb.Close()
		sqlDB, err := db.DB()
		if err == nil && sqlDB != nil {
			_ = sqlDB.Close()
		}
	}

	return db, rdb, cleanup, nil
}

func SetGinMode(mode string) {
	switch mode {
	case "prod", "release":
		gin.SetMode(gin.ReleaseMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}
}

func MustClose(name string, fn func() error) {
	if err := fn(); err != nil {
		log.Printf("close %s failed: %v", name, err)
	}
}
