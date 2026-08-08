package model

import "time"

type Order struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	ProductID uint      `gorm:"index;not null" json:"product_id"`
	Amount    float64   `gorm:"not null" json:"amount"`
	Status    string    `gorm:"size:16;default:pending;not null;index:idx_orders_status_created,priority:1" json:"status"`
	Provider  string    `gorm:"size:32;default:mock" json:"provider"`
	PaymentURL string   `gorm:"size:512" json:"payment_url"`
	CreatedAt time.Time `gorm:"index:idx_orders_status_created,priority:2" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Order) TableName() string {
	return "orders"
}

type CreateOrderInput struct {
	ProductID uint   `json:"product_id" validate:"required"`
	Provider  string `json:"provider"`
}
