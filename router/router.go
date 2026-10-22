package router

import (
	"log"
	"mall/container"
	_ "mall/docs" // swagger docs
	"mall/middleware"
	"mall/pkg/ratelimit"
	"os"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"mall/pkg/core"
)

// Version 已迁移到 pkg/core.Version，见 core.Version
func InitRouter(container *container.Container) *gin.Engine {
	r := gin.New()
	// 恢复 panic：对外只回通用 500 JSON（含 request_id），堆栈写日志
	r.Use(middleware.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Tracing())
	r.Use(middleware.AccessLog())
	security := container.Config.GetSecurityConfig()
	r.Use(middleware.CORS(security.CORSOrigins, false))
	r.Static("/static", resolveStaticDir())

	// 只信任本机反向代理传来的 X-Forwarded-For，避免客户端伪造 IP 绕过限流
	if err := r.SetTrustedProxies(security.TrustedProxies); err != nil {
		log.Printf("set trusted proxies failed: %v", err)
	}

	// 统一 JSON 404 / 405，替代 gin 默认的 text/plain "404 page not found"
	r.NoRoute(middleware.NoRoute())
	r.NoMethod(middleware.NoMethod())

	api := r.Group("/api/v1")
	{
		api.GET("/ping", func(c *gin.Context) {
			core.SuccessWithMessage(c, "pong", nil)
		})
		api.GET("/meta", container.MetaHandler.Get)

		// 商品分类字典（前端不再硬编码）
		api.GET("/categories", container.CategoryHandler.List)

		products := api.Group("/products")
		{
			products.GET("", container.ProductHandler.ListPublic)
			products.GET("/search", container.ProductHandler.SearchPublic)
			// 兼容旧调用方：/products/categories 不再误落到 /products/:id
			products.GET("/categories", container.CategoryHandler.List)
			products.GET("/:id", container.ProductHandler.GetPublic)
		}

		users := api.Group("/users")
		{
			users.POST("/register", container.UserHandler.Register)
			users.POST("/login", container.UserHandler.Login)
			users.POST("/refresh-token", container.UserHandler.RefreshToken)
			users.POST("/logout", container.UserHandler.Logout)

			usersAuth := users.Group("")
			usersAuth.Use(middleware.JWTAuth(container.JWTSecret))
			{
				usersAuth.GET("/me", container.UserHandler.GetProfile)
				usersAuth.PUT("/me", container.UserHandler.Update)
				usersAuth.PUT("/me/password", container.UserHandler.ChangePassword)
				usersAuth.PUT("/me/pay-password", container.UserHandler.SetPayPassword)
				usersAuth.GET("/me/amount", container.UserHandler.GetAmount)

				usersAuth.POST("/addresses", container.AddressHandler.Create)
				usersAuth.GET("/addresses", container.AddressHandler.List)
				usersAuth.GET("/addresses/detail", container.AddressHandler.Get)
				usersAuth.PUT("/addresses", container.AddressHandler.Update)
				usersAuth.DELETE("/addresses", container.AddressHandler.Delete)
				usersAuth.PUT("/addresses/default", container.AddressHandler.SetDefault)

				usersAuth.POST("/favorite/folders", container.FavoriteHandler.CreateFolder)
				usersAuth.GET("/favorite/folders", container.FavoriteHandler.ListFolders)
				usersAuth.PUT("/favorite/folders", container.FavoriteHandler.UpdateFolder)
				usersAuth.DELETE("/favorite/folders", container.FavoriteHandler.DeleteFolder)
				usersAuth.POST("/favorite/items", container.FavoriteHandler.AddItem)
				usersAuth.DELETE("/favorite/items", container.FavoriteHandler.DeleteItem)
				usersAuth.GET("/favorite/items", container.FavoriteHandler.ListItems)
				usersAuth.GET("/favorite/check", container.FavoriteHandler.Check)

				usersAuth.POST("/cart/items", container.CartHandler.AddItem)
				usersAuth.GET("/cart/items", container.CartHandler.ListItems)
				usersAuth.PUT("/cart/items", container.CartHandler.UpdateItem)
				usersAuth.DELETE("/cart/items", container.CartHandler.DeleteItem)

				usersAuth.DELETE("/cart", container.CartHandler.Clear)

				usersAuth.PUT("/cart/items/checked", container.CartHandler.SetItemChecked)
				usersAuth.PUT("/cart/checked", container.CartHandler.SetAllChecked)
				usersAuth.GET("/cart/checked", container.CartHandler.ListCheckedItems)

				usersAuth.POST("/orders/cart", container.OrderHandler.CreateFromCart)
				usersAuth.POST("/orders/buy-now", container.OrderHandler.BuyNow)
				usersAuth.GET("/orders", container.OrderHandler.ListOrders)

				usersAuth.POST("/payments", container.PaymentHandler.CreatePayment)
				usersAuth.GET("/payments/detail", container.PaymentHandler.GetPayment)
				// 模拟支付仅在 DEMO_MODE 下挂载，避免公网裸奔被当成支付漏洞
				if security.DemoMode {
					usersAuth.POST("/payments/mock/success", container.PaymentHandler.MockPaySuccess)
				}

			}
		}
		seckillCfg := container.Config.GetSeckillConfig()
		seckillBucket := ratelimit.NewTokenBucket(seckillCfg.RateLimitRate, seckillCfg.RateLimitCapacity)
		// 秒杀活动列表：公开，无需登录
		api.GET("/seckill/activities", container.SeckillHandler.ListActivities)
		seckill := api.Group("/seckill")
		seckill.Use(middleware.TokenBucketLimit(seckillBucket))
		seckill.Use(middleware.JWTAuth(container.JWTSecret))
		{
			seckill.POST("/activities/:activity_id/buy", container.SeckillHandler.Buy)
			seckill.GET("/activities/:activity_id/my-result", container.SeckillHandler.MyResult)
		}
		admin := api.Group("/admin")
		admin.Use(middleware.JWTAuth(container.JWTSecret))
		admin.Use(middleware.AdminAuth())
		{
			admin.POST("/seckill/activities/init-stock", container.SeckillHandler.InitStock)
			admin.POST("/products", container.ProductHandler.Create)
			admin.GET("/products", container.ProductHandler.ListAdmin)
			admin.PUT("/products/detail", container.ProductHandler.UpdateProduct)
			admin.DELETE("/products/detail", container.ProductHandler.DeleteProduct)
			admin.POST("/products/reindex", container.ProductHandler.Reindex)
			admin.GET("/dead-letters/products", container.DeadLetterHandler.ListProductDeadLetters)
		}
	}

	// Swagger UI
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	return r

}

func resolveStaticDir() string {
	candidates := []string{"static", "../static"}
	for _, path := range candidates {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			return path
		}
	}
	return "static"
}
