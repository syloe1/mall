package core

import (
	"fmt"
	"mall/config"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	otelgorm "gorm.io/plugin/opentelemetry/tracing"
)

func InitMysql(cfg config.MysqlConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Dbname,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
		//只打印慢查询和错误（生产环境用）
	})
	if err != nil {
		return nil, err
	}
	//接入 OpenTelemetry 链路追踪
	if err := db.Use(otelgorm.NewPlugin()); err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)  // 最大空闲连接
	// 单进程最大连接数：2C2G 小机器上 60 已足够（Web + Worker 两个进程 × 60 < MySQL 上限 200），
	// 之前 100 会在压测/高并发下打满 MySQL max_connections 报 Error 1040。
	sqlDB.SetMaxOpenConns(60)  // 最大同时连接数
	sqlDB.SetConnMaxLifetime(time.Hour)        // 连接最大生命周期
	sqlDB.SetConnMaxIdleTime(30 * time.Minute) // 空闲最大时间

	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}
