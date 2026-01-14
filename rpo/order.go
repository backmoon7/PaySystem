package rpo

import (
	"gorm.io/gorm"
)

type Order struct {
	gorm.Model
	OrderID       string  `gorm:"uniqueIndex;not null"`
	UserID        uint    `gorm:"not null"`
	Amount        float64 `gorm:"not null"`
	Currency      string  `gorm:"not null"`
	Status        string  `gorm:"not null"`
	PaymentMethod string  `gorm:"not null"`
}
