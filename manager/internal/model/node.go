package model

import "time"

type Node struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:128;not null" json:"name"`
	Type        string    `gorm:"size:16;not null" json:"type"`        // v2ray / xray
	Address     string    `gorm:"size:255;not null" json:"address"`
	Port        int       `gorm:"not null" json:"port"`
	Protocol    string    `gorm:"size:32;not null" json:"protocol"`   // e.g. vless, vmess, shadowsocks, trojan
	Status      string    `gorm:"size:16;default:inactive;not null" json:"status"` // active / inactive / disabled
	TrafficUp   int64     `gorm:"default:0" json:"traffic_up"`        // bytes
	TrafficDown int64     `gorm:"default:0" json:"traffic_down"`      // bytes
	UserID      uint      `gorm:"index;not null" json:"user_id"`      // owner FK
	// Transport / security options used when generating node configs
	Network         string `gorm:"size:16;default:ws" json:"network"`               // ws / tcp / grpc / kcp
	Security        string `gorm:"size:16;default:none" json:"security"`            // none / tls / reality
	WSPath          string `gorm:"size:128" json:"ws_path"`                         // websocket path
	ServerName      string `gorm:"size:128" json:"server_name"`                     // SNI for TLS/REALITY
	RealtyPublicKey string `gorm:"size:128" json:"reality_public_key"`              // REALITY dest public key
	RealtyShortID   string `gorm:"size:64" json:"reality_short_id"`                 // REALITY short id
	Token           string `gorm:"uniqueIndex;size:128" json:"-"`                   // node daemon auth token
	LastHeartbeat   *time.Time `json:"last_heartbeat,omitempty"`                    // last daemon sync/heartbeat
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (Node) TableName() string {
	return "nodes"
}
