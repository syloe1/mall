//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"

	"mall/config"
	"mall/dao"
	"mall/database"
	"mall/model"
	"mall/pkg/core"
	"mall/service"
)

// TestEnv holds all dependencies for integration tests.
type TestEnv struct {
	DB  *gorm.DB
	RDB *redis.Client

	UserService    *service.UserService
	ProductService *service.ProductService
	CartService    *service.CartService
	OrderService   *service.OrderService
	PaymentService *service.PaymentService
	SeckillService *service.SeckillService

	UserID    uint
	ProductID uint
	cleanup   func()
}

// NewTestEnv connects to Docker services and initializes all stores.
func NewTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	// Connect MySQL (Docker: localhost:3308)
	mysqlCfg := config.MysqlConfig{
		Host:     "localhost",
		Port:     "3308",
		Dbname:   "mall",
		Username: "root",
		Password: "123456",
	}
	db, err := core.InitMysql(mysqlCfg)
	if err != nil {
		t.Fatalf("connect mysql failed: %v", err)
	}

	// Connect Redis (Docker: localhost:6379)
	redisCfg := config.RedisConfig{
		Host:     "localhost",
		Port:     "6379",
		Password: "",
		DB:       0,
	}
	rdb := core.InitRedis(redisCfg)

	// Auto-migrate
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	// DAOs
	userDB := &dao.GormUserDB{DB: db}
	productDB := &dao.GormProductDB{DB: db}
	cartDB := &dao.GormCartDB{DB: db}
	orderDB := &dao.GormOrderDB{DB: db}
	paymentDB := &dao.GormPaymentDB{DB: db}
	seckillDB := &dao.GormSeckillDB{DB: db}

	jwtCfg := &dao.DefaultJWTConfig{Secret: []byte("integration-test-secret")}
	jwtSecret := []byte("integration-test-secret")

	// Services
	userSvc := service.NewUserService(userDB, jwtCfg, jwtSecret, rdb)
	productSvc := service.NewProductService(productDB, nil)
	cartSvc := service.NewCartService(cartDB, productDB)
	orderSvc := service.NewOrderService(db, rdb, nil, orderDB, cartDB, productDB, &dao.GormAddressDB{DB: db})
	paymentSvc := service.NewPaymentService(paymentDB, orderDB, userDB, rdb)
	seckillSvc := service.NewSeckillService(rdb, seckillDB, nil)

	cleanup := func() {
		_ = rdb.Close()
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}

	return &TestEnv{
		DB:             db,
		RDB:            rdb,
		UserService:    userSvc,
		ProductService: productSvc,
		CartService:    cartSvc,
		OrderService:   orderSvc,
		PaymentService: paymentSvc,
		SeckillService: seckillSvc,
		cleanup:        cleanup,
	}
}

func (e *TestEnv) Close() {
	if e.cleanup != nil {
		e.cleanup()
	}
}

// seedUser creates a test user. Returns userID.
func (e *TestEnv) seedUser(t *testing.T, username, password string) uint {
	t.Helper()
	err := e.UserService.Register(username, password)
	if err != nil {
		t.Fatalf("seed user failed: %v", err)
	}
	var user model.User
	if err := e.DB.Where("username = ?", username).First(&user).Error; err != nil {
		t.Fatalf("find seeded user failed: %v", err)
	}
	return user.ID
}

// seedProduct creates a test product. Returns productID.
func (e *TestEnv) seedProduct(t *testing.T) uint {
	t.Helper()
	p := &model.Product{
		Name:       "集成测试商品",
		PriceCents: 9900,
		Stock:      100,
		Status:     model.ProductOnSale,
		CategoryID: 1,
	}
	if err := e.DB.Create(p).Error; err != nil {
		t.Fatalf("seed product failed: %v", err)
	}
	return p.ID
}

// CleanTable truncates specified tables between tests.
func (e *TestEnv) CleanTable(t *testing.T, tables ...string) {
	t.Helper()
	for _, table := range tables {
		e.DB.Exec("DELETE FROM " + table)
	}
}

// CleanAll truncates all tables.
func (e *TestEnv) CleanAll(t *testing.T) {
	t.Helper()
	// Disable foreign key checks for truncation
	e.DB.Exec("SET FOREIGN_KEY_CHECKS = 0")
	tables := []string{
		"payments", "order_items", "orders",
		"cart_items",
		"favorite_items", "favorite_folders",
		"user_addresses",
		"seckill_purchases", "seckill_activities",
		"product_images", "products",
		"users",
	}
	for _, table := range tables {
		e.DB.Exec("DELETE FROM " + table)
	}
	e.DB.Exec("SET FOREIGN_KEY_CHECKS = 1")

	// Clean Redis
	e.RDB.FlushDB(context.Background())
}
