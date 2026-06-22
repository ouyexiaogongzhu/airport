package model

import "time"

type Node struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:128;not null" json:"name"`
	Type        string    `gorm:"size:16;not null" json:"type"`        // v2ray / xray
	Address     string    `gorm:"size:255;not null" json:"address"`
	Port        int       `gorm:"not null" json:"port"`
	Protocol    string    `gorm:"size:32;not null" json:"protocol"`   // e.g. vmess, shadowsocks, trojan
	Status      string    `gorm:"size:16;default:inactive;not null" json:"status"` // active / inactive / disabled
	TrafficUp   int64     `gorm:"default:0" json:"traffic_up"`        // bytes
	TrafficDown int64     `gorm:"default:0" json:"traffic_down"`      // bytes
	UserID      uint      `gorm:"index;not null" json:"user_id"`      // owner FK
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (Node) TableName() string {
	return "nodes"
}
