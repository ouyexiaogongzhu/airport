package model

import "time"

type Product struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128;not null" json:"name"`
	Type      string    `gorm:"size:32;not null" json:"type"`
	Price     float64   `gorm:"not null" json:"price"`
	Stock     int       `gorm:"default:0" json:"stock"`
	Status    string    `gorm:"size:16;default:active;not null" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Product) TableName() string {
	return "products"
}
