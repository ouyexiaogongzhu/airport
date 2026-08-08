package model

import "time"

type TrafficRecord struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	NodeID       uint      `gorm:"index;index:idx_traffic_node_time,priority:1;not null" json:"node_id"`
	UserID       uint      `gorm:"index;index:idx_traffic_user_time,priority:1;not null" json:"user_id"`
	UploadBytes  int64     `gorm:"not null" json:"upload_bytes"`
	DownloadBytes int64    `gorm:"not null" json:"download_bytes"`
	RecordedAt   time.Time `gorm:"index;index:idx_traffic_node_time,priority:2;index:idx_traffic_user_time,priority:2;not null" json:"recorded_at"`
}

func (TrafficRecord) TableName() string {
	return "traffic_records"
}
