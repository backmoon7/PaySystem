package model

import (
	"time"
)

const (
	OutboxStatusPending = "PENDING"
	OutboxStatusSent    = "SENT"
	OutboxStatusFailed  = "FAILED"
)

type OutboxMessage struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	MessageKey string    `gorm:"type:varchar(64);not null" json:"message_key"`
	Topic      string    `gorm:"type:varchar(64);not null" json:"topic"`
	Payload    string    `gorm:"type:text;not null" json:"payload"`
	Status     string    `gorm:"type:varchar(20);index;not null;default:PENDING" json:"status"`
	RetryCount int       `gorm:"not null;default:0" json:"retry_count"`
	CreatedAt  time.Time `gorm:"autoCreateTime;index" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (OutboxMessage) TableName() string {
	return "outbox_message"
}
