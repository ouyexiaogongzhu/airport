package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
)

type ReportTrafficRequest struct {
	NodeID         uint  `json:"node_id" validate:"required"`
	UserID         uint  `json:"user_id" validate:"required"`
	UploadBytes    int64 `json:"upload_bytes" validate:"required"`
	DownloadBytes  int64 `json:"download_bytes" validate:"required"`
}

type TrafficStatsQuery struct {
	UserID uint   `json:"user_id"`
	NodeID uint   `json:"node_id"`
	Since  string `json:"since"`  // RFC3339 or empty
	Until  string `json:"until"`
}

// ReportTraffic records a traffic data point and updates the node's cumulative counters.
func ReportTraffic(c *fiber.Ctx) error {
	req := new(ReportTrafficRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.NodeID == 0 || req.UserID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "node_id and user_id are required",
		})
	}

	// Verify the node exists
	var node model.Node
	if result := db.DB.First(&node, req.NodeID); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "node not found",
		})
	}

	// Record traffic data point
	record := model.TrafficRecord{
		NodeID:        req.NodeID,
		UserID:        req.UserID,
		UploadBytes:   req.UploadBytes,
		DownloadBytes: req.DownloadBytes,
		RecordedAt:    time.Now(),
	}
	if result := db.DB.Create(&record); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to record traffic",
		})
	}

	// Update node cumulative counters
	db.DB.Model(&node).Updates(map[string]interface{}{
		"traffic_up":   node.TrafficUp + req.UploadBytes,
		"traffic_down": node.TrafficDown + req.DownloadBytes,
	})

	return c.Status(fiber.StatusCreated).JSON(record)
}

// TrafficSummary is the aggregated result row.
type TrafficSummary struct {
	NodeID          uint   `json:"node_id"`
	UserID          uint   `json:"user_id,omitempty"`
	TotalUpload     int64  `json:"total_upload"`
	TotalDownload   int64  `json:"total_download"`
}

// GetTrafficStats returns aggregated traffic statistics.
// Query params: user_id, node_id, since, until
func GetTrafficStats(c *fiber.Ctx) error {
	var req TrafficStatsQuery
	req.UserID = uint(c.QueryInt("user_id", 0))
	req.NodeID = uint(c.QueryInt("node_id", 0))
	req.Since = c.Query("since", "")
	req.Until = c.Query("until", "")

	query := db.DB.Model(&model.TrafficRecord{})

	if req.NodeID > 0 {
		query = query.Where("node_id = ?", req.NodeID)
	}
	if req.UserID > 0 {
		query = query.Where("user_id = ?", req.UserID)
	}
	if req.Since != "" {
		t, err := time.Parse(time.RFC3339, req.Since)
		if err == nil {
			query = query.Where("recorded_at >= ?", t)
		}
	}
	if req.Until != "" {
		t, err := time.Parse(time.RFC3339, req.Until)
		if err == nil {
			query = query.Where("recorded_at <= ?", t)
		}
	}

	// Aggregate by node_id, user_id
	type resultRow struct {
		NodeID        uint  `gorm:"column:node_id"`
		UserID        uint  `gorm:"column:user_id"`
		TotalUpload   int64 `gorm:"column:total_upload"`
		TotalDownload int64 `gorm:"column:total_download"`
	}

	var results []resultRow
	if err := query.Select("node_id, user_id, SUM(upload_bytes) AS total_upload, SUM(download_bytes) AS total_download").
		Group("node_id, user_id").
		Scan(&results).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to query traffic stats",
		})
	}

	// Convert to response
	summaries := make([]TrafficSummary, 0, len(results))
	for _, r := range results {
		summaries = append(summaries, TrafficSummary{
			NodeID:        r.NodeID,
			UserID:        r.UserID,
			TotalUpload:   r.TotalUpload,
			TotalDownload: r.TotalDownload,
		})
	}

	return c.JSON(fiber.Map{
		"data": summaries,
	})
}
