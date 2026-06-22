package model

import "time"

type User struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	Username            string    `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash        string    `gorm:"size:255;not null" json:"-"`
	Role                string    `gorm:"size:32;default:user;not null" json:"role"`
	Balance             float64   `gorm:"default:0" json:"balance"`
	Status              string    `gorm:"size:16;default:active;not null" json:"status"`
	ClientToken         string    `gorm:"uniqueIndex;size:128" json:"client_token"`
	SubscriptionStatus  string    `gorm:"size:16;default:pending" json:"subscription_status"` // pending/active/expired/disabled
	SubscriptionTier    string    `gorm:"size:32" json:"subscription_tier"`
	TrafficLimitBytes   int64     `gorm:"default:0" json:"traffic_limit_bytes"`
	TrafficUsedBytes    int64     `gorm:"default:0" json:"traffic_used_bytes"`
	ExpireTime          int64     `gorm:"default:0" json:"expire_time"`
	RateLimitBps        int64     `gorm:"default:0" json:"rate_limit_bps"`
	TrafficPeriodStart  int64     `gorm:"default:0" json:"traffic_period_start"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (User) TableName() string {
	return "users"
}
