package model

import "time"

type TrafficRecord struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	NodeID       uint      `gorm:"index;not null" json:"node_id"`
	UserID       uint      `gorm:"index;not null" json:"user_id"`
	UploadBytes  int64     `gorm:"not null" json:"upload_bytes"`
	DownloadBytes int64    `gorm:"not null" json:"download_bytes"`
	RecordedAt   time.Time `gorm:"index;not null" json:"recorded_at"`
}

func (TrafficRecord) TableName() string {
	return "traffic_records"
}
