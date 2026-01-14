package handler

import (
	"paysystem/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// SetupRouter 配置路由
func SetupRouter(db *gorm.DB, rdb *redis.Client, cfg *config.Config) *gin.Engine {
	// 设置 gin 为发布模式（减少日志输出）
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()

	// 注册中间件
	r.Use(RecoveryMiddleware())
	r.Use(LoggerMiddleware())
	r.Use(CORSMiddleware())

	// 创建处理器
	h := NewHandler(db, rdb, cfg)

	// API 路由组
	api := r.Group("/api/v1")
	{
		// 账户相关
		account := api.Group("/account")
		{
			account.GET("/balance", h.GetBalance)
			account.POST("/recharge", h.Recharge)
		}

		// 订单相关
		order := api.Group("/order")
		{
			order.POST("/create", h.CreateOrder)
			order.GET("/detail", h.GetOrder)
			order.GET("/list", h.ListOrders)
			order.POST("/cancel", h.CancelOrder)
		}

		// 支付相关
		pay := api.Group("/pay")
		{
			pay.POST("/execute", h.PayOrder)
		}

		// 退款相关
		refund := api.Group("/refund")
		{
			refund.POST("/execute", h.RefundOrder)
		}
	}

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return r
}
