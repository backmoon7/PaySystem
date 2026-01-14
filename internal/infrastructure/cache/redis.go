package cache

import (
	"context"
	"fmt"
	"log"
	"time"

	"paysystem/internal/config"

	"github.com/go-redis/redis/v8"
)

var RedisClient *redis.Client

func InitRedis(cfg *config.RedisConfig) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("连接 Redis 失败: %v", err)
	}

	RedisClient = client
	log.Println("Redis 连接成功")
	return client
}
