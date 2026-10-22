package main

// @title           Mall 电商系统 API
// @version         1.0
// @description     商城后端接口文档，包含用户、商品、购物车、订单、支付、秒杀等模块。
// @host            localhost:3333
// @BasePath        /api/v1
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description  JWT Token，格式：Bearer <token>

import (
	"context"
	"log"
	"mall/container"
	"mall/pkg/bootstrap"
	"mall/pkg/logx"
	"mall/pkg/mq"
	"mall/router"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logx.Init()

	cfg, err := bootstrap.LoadWebConfig()
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}
	// 安全基线：JWT 密钥为空时任何人都能自签 token，生产必须拒绝启动
	if cfg.GetJwtConfig().Secret == "" {
		log.Fatal("JWT secret is empty: 请通过 JWT_SECRET 环境变量或配置文件注入至少 32 位随机串")
	}
	if cfg.GetServerConfig().Model == "prod" && os.Getenv("AES_KEY") == "" {
		log.Fatal("AES_KEY is required in production（余额字段使用 AES 加密）")
	}
	//初始化链路追踪
	shutdownTrace, err := bootstrap.InitTrace(cfg)
	if err != nil {
		log.Fatalf("init tracing failed: %v", err)
	}
	defer func() {
		_ = shutdownTrace(context.Background())
	}()

	bootstrap.SetGinMode(cfg.GetServerConfig().Model)
	//初始化所有存储服务
	db, rdb, cleanup, err := bootstrap.InitStore(cfg)
	if err != nil {
		log.Fatalf("init store failed: %v", err)
	}
	defer cleanup()
	//连接RabbitMQ
	rmq, err := mq.NewRabbitMQ(cfg.GetRabbitMQConfig().URL)
	if err != nil {
		log.Fatalf("connect rabbitmq failed: %v", err)
	}
	defer rmq.Close()
	//订单延时队列
	//30分钟未支付， 取消订单
	ch, err := rmq.Channel()
	if err != nil {
		log.Fatalf("create rabbitmq channel failed: %v", err)
	}
	if err := mq.DeclareOrderTimeoutQueues(ch); err != nil {
		log.Fatalf("declare order queues failed: %v", err)
	}
	if err := mq.DeclareSeckillQueues(ch); err != nil {
		log.Fatalf("declare seckill queues failed: %v", err)
	}
	_ = ch.Close()
	//创建订单消息生产者
	orderProducer, err := mq.NewProducer(rmq, mq.OrderDelayExchange)
	if err != nil {
		log.Fatalf("create order producer failed: %v", err)
	}
	defer orderProducer.Close()

	seckillProducer, err := mq.NewProducer(rmq, mq.SeckillExchange)
	if err != nil {
		log.Fatalf("create seckill producer failed: %v", err)
	}
	defer seckillProducer.Close()

	//配置
	//MySQL
	//Meilisearch（搜索）
	//Redis
	//消息生产者
	cont := container.NewContainer(cfg, db, rdb, orderProducer, seckillProducer)

	orderConsumer, err := mq.NewConsumer(rmq)
	if err != nil {
		log.Fatalf("create order consumer failed: %v", err)
	}
	defer orderConsumer.Close()
	//启动订单超时消费者（
	if err := orderConsumer.Start(mq.OrderTimeoutQueue, 5, func(ctx context.Context, msg mq.Message) error {
		if msg.Event != "order.created" {
			return nil
		}
		return cont.OrderService.CancelTimeoutOrder(msg.UserID, msg.BizID)
	}); err != nil {
		log.Fatalf("start order timeout consumer failed: %v", err)
	}

	r := router.InitRouter(cont)

	srv := &http.Server{
		Addr:    ":" + cfg.GetServerConfig().Port,
		Handler: r,
	}

	// 优雅关闭：收到 SIGINT/SIGTERM 后，等 10s 让现有请求处理完再退出
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("server forced to shutdown: %v", err)
		}
	}()

	log.Printf("server starting on :%s", cfg.GetServerConfig().Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("start server failed: %v", err)
	}
	log.Println("server exited")
}
