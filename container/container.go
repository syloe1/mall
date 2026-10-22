package container

import (
	"log"
	"mall/config"
	"mall/dao"
	"mall/handler"
	"mall/middleware"
	"mall/pkg/core"
	"mall/pkg/mq"
	"mall/service"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type Container struct {
	Config              *config.App
	Logger              *log.Logger
	DB                  *gorm.DB
	JWTSecret           []byte
	Redis               *redis.Client
	ProductSearchDB     dao.ProductSearchDB
	ProductOutboxDB     dao.ProductOutboxDB
	ProductDeadLetterDB dao.ProductDeadLetterDB
	FavoriteDB          dao.FavoriteDB
	AddressDB           dao.AddressDB
	CartDB              dao.CartDB
	OrderDB             dao.OrderDB
	PaymentDB           dao.PaymentDB
	SeckillDB           dao.SeckillDB
	CategoryDB          dao.CategoryDB

	UserService *service.UserService
	UserHandler *handler.UserHandler

	ProductService         *service.ProductService
	ProductHandler         *handler.ProductHandler

	CategoryService *service.CategoryService
	CategoryHandler *handler.CategoryHandler

	MetaHandler *handler.MetaHandler

	DeadLetterService *service.DeadLetterService
	DeadLetterHandler *handler.DeadLetterHandler

	FavoriteService *service.FavoriteService
	FavoriteHandler *handler.FavoriteHandler

	AddressService *service.AddressService
	AddressHandler *handler.AddressHandler

	CartService *service.CartService
	CartHandler *handler.CartHandler

	OrderService *service.OrderService
	OrderHandler *handler.OrderHandler

	PaymentService *service.PaymentService
	PaymentHandler *handler.PaymentHandler

	SeckillService *service.SeckillService
	SeckillHandler *handler.SeckillHandler
}

func NewContainer(cfg *config.App, db *gorm.DB, rdb *redis.Client, orderProducer *mq.Producer, seckillProducer *mq.Producer) *Container {
	jwtSecret := []byte(cfg.GetJwtConfig().Secret)
	userDB := &dao.GormUserDB{DB: db}
	productDB := &dao.GormProductDB{DB: db}
	productOutboxDB := &dao.GormProductOutboxDB{DB: db}
	productDeadLetterDB := &dao.GormProductDeadLetterDB{DB: db}
	favoriteDB := &dao.GormFavoriteDB{DB: db}
	productSearchDB := dao.NewMeiliProductDB(
		cfg.GetMeilisearchConfig().Host,
		cfg.GetMeilisearchConfig().APIKey,
		cfg.GetMeilisearchConfig().ProductIndex,
	)
	addressDB := &dao.GormAddressDB{DB: db}
	cartDB := &dao.GormCartDB{DB: db}
	orderDB := &dao.GormOrderDB{DB: db}
	paymentDB := &dao.GormPaymentDB{DB: db}
	seckillDB := &dao.GormSeckillDB{DB: db}
	categoryDB := &dao.GormCategoryDB{DB: db}

	jwtCfg := &dao.DefaultJWTConfig{Secret: jwtSecret}

	userService := service.NewUserService(userDB, jwtCfg, jwtSecret, rdb)
	userHandler := handler.NewUserHandler(userService, middleware.NewLoginLimiter(rdb), cfg)

	productService := service.NewProductService(productDB, productSearchDB)
	productHandler := handler.NewProductHandler(productService)

	categoryService := service.NewCategoryService(categoryDB)
	categoryHandler := handler.NewCategoryHandler(categoryService)

	metaHandler := handler.NewMetaHandler(cfg.GetSecurityConfig().DemoMode, core.Version)
	deadLetterService := service.NewDeadLetterService(productDeadLetterDB)
	deadLetterHandler := handler.NewDeadLetterHandler(deadLetterService)

	favoriteService := service.NewFavoriteService(favoriteDB, productDB)
	favoriteHandler := handler.NewFavoriteHandler(favoriteService)

	addressService := service.NewAddressService(addressDB)
	addressHandler := handler.NewAddressHandler(addressService)

	cartService := service.NewCartService(cartDB, productDB)
	cartHandler := handler.NewCartHandler(cartService)

	orderService := service.NewOrderService(db, rdb, orderProducer, orderDB, cartDB, productDB, addressDB)
	orderHandler := handler.NewOrderHandler(orderService)

	paymentService := service.NewPaymentService(paymentDB, orderDB, userDB, rdb)
	paymentHandler := handler.NewPaymentHandler(paymentService)
	seckillService := service.NewSeckillService(rdb, seckillDB, seckillProducer)
	seckillHandler := handler.NewSeckillHandler(seckillService)

	return &Container{
		Config:                 cfg,
		DB:                     db,
		Logger:                 log.Default(),
		JWTSecret:              jwtSecret,
		Redis:                  rdb,
		ProductSearchDB:        productSearchDB,
		ProductOutboxDB:        productOutboxDB,
		ProductDeadLetterDB:    productDeadLetterDB,
		FavoriteDB:             favoriteDB,
		AddressDB:              addressDB,
		CartDB:                 cartDB,
		OrderDB:                orderDB,
		PaymentDB:              paymentDB,
		SeckillDB:              seckillDB,
		CategoryDB:             categoryDB,
		UserService:            userService,
		UserHandler:            userHandler,
		ProductService:         productService,
		ProductHandler:         productHandler,
		CategoryService:        categoryService,
		CategoryHandler:        categoryHandler,
		MetaHandler:            metaHandler,
		DeadLetterService:      deadLetterService,
		DeadLetterHandler:      deadLetterHandler,
		FavoriteService:        favoriteService,
		FavoriteHandler:        favoriteHandler,
		AddressService:         addressService,
		AddressHandler:         addressHandler,
		CartService:            cartService,
		CartHandler:            cartHandler,
		OrderService:           orderService,
		OrderHandler:           orderHandler,
		PaymentService:         paymentService,
		PaymentHandler:         paymentHandler,
		SeckillService:         seckillService,
		SeckillHandler:         seckillHandler,
	}
}
