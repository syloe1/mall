package main

import (
	"context"
	"log"
	"log/slog"
	"mall/dao"
	"mall/pkg/bootstrap"
	"mall/pkg/logx"
	"mall/pkg/mq"
	"mall/service"
	"os/signal"
	"syscall"
)

/*
后台任务（worker）进程：
1. 轮询数据库 product_outbox 表 → 发送商品事件到 RabbitMQ
2. 监听 RabbitMQ 商品事件队列 → 同步商品数据到 Meilisearch（搜索用）
3. 监听 RabbitMQ 秒杀队列 → 创建秒杀订单，并发 30 分钟延迟关单消息（由 web 进程的订单超时消费者取消）
*/

func main() {
	logx.Init()
	slog.Info("mall worker starting")

	cfg, err := bootstrap.LoadWorkerConfig()
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}
	//链路追踪
	shutdownTrace, err := bootstrap.InitTrace(cfg)
	if err != nil {
		log.Fatalf("init tracing failed: %v", err)
	}
	defer func() {
		_ = shutdownTrace(context.Background())
	}()
	//初始化所有存储服务
	db, rdb, cleanup, err := bootstrap.InitStore(cfg)
	if err != nil {
		log.Fatalf("init store failed: %v", err)
	}
	defer cleanup()

	rmq, err := mq.NewRabbitMQ(cfg.GetRabbitMQConfig().URL)
	if err != nil {
		log.Fatalf("connect rabbitmq failed: %v", err)
	}
	defer rmq.Close()

	//声明交换机/队列（秒杀、商品事件）
	ch, err := rmq.Channel()
	if err != nil {
		log.Fatalf("create rabbitmq channel failed: %v", err)
	}
	if err := mq.DeclareSeckillQueues(ch); err != nil {
		log.Fatalf("declare seckill queues failed: %v", err)
	}
	if err := mq.DeclareProductQueues(ch); err != nil {
		log.Fatalf("declare product queues failed: %v", err)
	}
	//订单超时队列：worker 创建秒杀订单后要发延迟关单消息，这里必须先声明交换机和队列
	if err := mq.DeclareOrderTimeoutQueues(ch); err != nil {
		log.Fatalf("declare order timeout queues failed: %v", err)
	}
	_ = ch.Close()

	//商品事件生产者（outbox → RabbitMQ 商品交换机）
	productProducer, err := mq.NewProducer(rmq, mq.ProductExchange)
	if err != nil {
		log.Fatalf("create product producer failed: %v", err)
	}
	defer func() {
		_ = productProducer.Close()
	}()

	//订单延迟消息生产者（秒杀订单创建后发 30 分钟延迟关单消息 → 订单超时交换机）
	orderProducer, err := mq.NewProducer(rmq, mq.OrderDelayExchange)
	if err != nil {
		log.Fatalf("create order producer failed: %v", err)
	}
	defer func() {
		_ = orderProducer.Close()
	}()

	deadLetterDB := &dao.GormProductDeadLetterDB{DB: db}
	//消息发布者：轮询 product_outbox 表，读到消息就发到 RabbitMQ
	outboxPublisher := service.NewProductOutboxPublisher(
		&dao.GormProductOutboxDB{DB: db},
		deadLetterDB,
		service.NewRabbitProductProducer(productProducer),
	)

	//商品索引消费者：监听商品事件 → 同步到 Meilisearch
	productConsumer, err := mq.NewConsumer(rmq)
	if err != nil {
		log.Fatalf("create product consumer failed: %v", err)
	}
	defer productConsumer.Close()
	searchConsumer := service.NewProductSearchConsumer(
		productConsumer,
		&dao.GormProductDB{DB: db},
		dao.NewMeiliProductDB(
			cfg.GetMeilisearchConfig().Host,
			cfg.GetMeilisearchConfig().APIKey,
			cfg.GetMeilisearchConfig().ProductIndex,
		),
		deadLetterDB,
	)

	// 秒杀消费者需要把抢购成功落成真实订单：注入 OrderService
	orderDB := &dao.GormOrderDB{DB: db}
	cartDB := &dao.GormCartDB{DB: db}
	productDB := &dao.GormProductDB{DB: db}
	addressDB := &dao.GormAddressDB{DB: db}
	//第三个参数是消息生产者：秒杀订单也要发延迟关单消息，之前传 nil 导致秒杀单永远不会被超时取消
	orderService := service.NewOrderService(db, rdb, orderProducer, orderDB, cartDB, productDB, addressDB)

	seckillService := service.NewSeckillService(rdb, &dao.GormSeckillDB{DB: db}, nil)
	seckillService.SetOrderCreator(orderService)
	seckillConsumer, err := mq.NewConsumer(rmq)
	if err != nil {
		log.Fatalf("create seckill consumer failed: %v", err)
	}
	defer seckillConsumer.Close()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,  // Ctrl+C
		syscall.SIGTERM, // 停止信号
	)
	defer stop()
	/*
		后台协程
		发消息给RabbitMQ
		从RabbitMQ消费消息
	*/
	go outboxPublisher.Start(ctx)
	if err := searchConsumer.Start(ctx); err != nil {
		log.Fatalf("start product search consumer failed: %v", err)
	}
	if err := seckillConsumer.Start(mq.SeckillOrderQueue, 20, func(ctx context.Context, msg mq.Message) error {
		if msg.Event != "seckill.order.create" {
			return nil
		}
		if err := seckillService.CreatePurchaseFromMessage(msg.BizID, msg.UserID); err != nil {
			slog.Error("create seckill purchase failed", "activity_id", msg.BizID, "user_id", msg.UserID, "error", err)
			return nil
		}
		slog.Info("seckill order created", "activity_id", msg.BizID, "user_id", msg.UserID)
		return nil
	}); err != nil {
		log.Fatalf("start seckill consumer failed: %v", err)
	}
	slog.Info("mall worker ready: outbox publisher, product search consumer, seckill consumer started")
	//阻塞等待关闭信号
	<-ctx.Done()
	slog.Info("mall worker stopping")
}
