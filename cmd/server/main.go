package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"paysystem/internal/config"
	"paysystem/internal/handler"
	"paysystem/internal/infrastructure/cache"
	"paysystem/internal/infrastructure/database"
	"paysystem/internal/infrastructure/mq"
	"paysystem/internal/job"
	"paysystem/pkg/idgen"
)

func main() {
	// 加载配置
	cfg := config.LoadConfig("config/config.yaml")

	// 初始化 ID 生成器
	idgen.Init(1)

	// 初始化 MySQL
	db := database.InitMySQL(&cfg.MySQL)

	// 初始化 Redis
	redisClient := cache.InitRedis(&cfg.Redis)

	// 初始化 Kafka
	mq.InitKafka(&cfg.Kafka)
	defer mq.CloseKafka()

	// 创建上下文（用于优雅关闭）
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动后台任务
	outboxSender := job.NewOutboxSender(db, cfg)
	go outboxSender.Start(ctx)

	orderTimeoutJob := job.NewOrderTimeoutJob(db, cfg)
	go orderTimeoutJob.Start(ctx)

	compensateJob := job.NewPayingOrderCompensateJob(db, cfg)
	go compensateJob.Start(ctx)

	// 设置路由
	router := handler.SetupRouter(db, redisClient, cfg)

	// 启动 HTTP 服务
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}

	// 在 goroutine 中启动服务器
	go func() {
		log.Printf("服务启动，监听端口: %d", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务启动失败: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务...")

	// 取消上下文，停止后台任务
	cancel()

	// 关闭 HTTP 服务（等待最多5秒）
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("服务关闭异常: %v", err)
	}

	log.Println("服务已关闭")
}
