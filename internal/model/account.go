package model

import (
	"time"
)

// Account 用户账户表
// 记录用户的硬币余额，是整个支付系统的核心数据
type Account struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       int64     `gorm:"uniqueIndex;not null" json:"user_id"`     // 用户ID，业务方传入
	Balance      int64     `gorm:"not null;default:0" json:"balance"`       // 可用余额（硬币数）
	FrozenAmount int64     `gorm:"not null;default:0" json:"frozen_amount"` // 冻结金额（预留，暂不使用）
	Version      int       `gorm:"not null;default:0" json:"version"`       // 乐观锁版本号
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Account) TableName() string {
	return "account"
}
