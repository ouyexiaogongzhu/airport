package model

import "time"

type Order struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	ProductID uint      `gorm:"index;not null" json:"product_id"`
	Amount    float64   `gorm:"not null" json:"amount"`
	Status    string    `gorm:"size:16;default:pending;not null" json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func (Order) TableName() string {
	return "orders"
}
