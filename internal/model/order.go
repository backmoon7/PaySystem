package model

import (
	"time"
)

const (
	OrderStatusCreated   = "CREATED"
	OrderStatusPaying    = "PAYING"
	OrderStatusPaid      = "PAID"
	OrderStatusFailed    = "FAILED"
	OrderStatusClosed    = "CLOSED"
	OrderStatusCancelled = "CANCELLED"
	OrderStatusRefunding = "REFUNDING"
	OrderStatusRefunded  = "REFUNDED"
)

var ValidStatusTransitions = map[string][]string{
	OrderStatusCreated:   {OrderStatusPaying, OrderStatusClosed, OrderStatusCancelled},
	OrderStatusPaying:    {OrderStatusPaid, OrderStatusFailed},
	OrderStatusPaid:      {OrderStatusRefunding},
	OrderStatusRefunding: {OrderStatusRefunded},
}

func CanTransitionTo(currentStatus, targetStatus string) bool {
	allowedStatuses, exists := ValidStatusTransitions[currentStatus]
	if !exists {
		return false
	}
	for _, s := range allowedStatuses {
		if s == targetStatus {
			return true
		}
	}
	return false
}

const (
	ProductTypeCoinVideo   = "COIN_VIDEO"
	ProductTypeCoinProduct = "COIN_PRODUCT"
)

type PayOrder struct {
	ID          int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	OrderNo     string     `gorm:"type:varchar(64);uniqueIndex;not null" json:"order_no"`
	RequestID   string     `gorm:"type:varchar(64);uniqueIndex;not null" json:"request_id"`
	UserID      int64      `gorm:"index;not null" json:"user_id"`
	Amount      int64      `gorm:"not null" json:"amount"`
	ProductType string     `gorm:"type:varchar(32);not null" json:"product_type"`
	ProductID   string     `gorm:"type:varchar(64);not null" json:"product_id"`
	Status      string     `gorm:"type:varchar(20);index;not null" json:"status"`
	ExpiredAt   time.Time  `gorm:"not null" json:"expired_at"`
	PaidAt      *time.Time `json:"paid_at"`
	CreatedAt   time.Time  `gorm:"autoCreateTime;index" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (PayOrder) TableName() string {
	return "pay_order"
}
