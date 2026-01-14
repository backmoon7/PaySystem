package database

import (
	"fmt"
	"log"
	"time"

	"paysystem/internal/config"
	"paysystem/internal/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitMySQL 初始化 MySQL 连接
func InitMySQL(cfg *config.MySQLConfig) *gorm.DB {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("连接 MySQL 失败: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("获取底层 DB 失败: %v", err)
	}

	// 连接池配置
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 自动迁移表结构
	err = db.AutoMigrate(
		&model.Account{},
		&model.PayOrder{},
		&model.AccountTransaction{},
		&model.OutboxMessage{},
	)
	if err != nil {
		log.Fatalf("自动迁移表结构失败: %v", err)
	}

	DB = db
	log.Println("MySQL 连接成功")
	return db
}
