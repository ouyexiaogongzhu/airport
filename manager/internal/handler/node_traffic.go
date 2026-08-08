package handler

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
	"gorm.io/gorm"
)

// maxNodeTrafficEntries caps a single traffic report batch so a compromised or
// misbehaving node cannot amplify DB writes or touch an unbounded user set in
// one request.
const maxNodeTrafficEntries = 1000

// NodeTrafficEntry is a single user's traffic delta reported by a node daemon.
type NodeTrafficEntry struct {
	UserID       uint  `json:"user_id" validate:"required"`
	UploadBytes  int64 `json:"upload_bytes"`
	DownloadBytes int64 `json:"download_bytes"`
}

// NodeTrafficReportRequest is the batch payload from a node daemon.
type NodeTrafficReportRequest struct {
	NodeID   uint               `json:"node_id" validate:"required"`
	Traffic  []NodeTrafficEntry `json:"traffic"`
}

// ReportNodeTraffic records per-user traffic reported by an authenticated node
// daemon (token + HMAC enforced by the NodeHMAC middleware).
// POST /api/v1/node/:token/traffic/report
func ReportNodeTraffic(c *fiber.Ctx) error {
	node, ok := c.Locals("node").(*model.Node)
	if !ok || node == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "INVALID_TOKEN"})
	}

	req := new(NodeTrafficReportRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.NodeID == 0 {
		req.NodeID = node.ID
	}
	if req.NodeID != node.ID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "node_id does not match token"})
	}
	if len(req.Traffic) > maxNodeTrafficEntries {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "too many traffic entries"})
	}

	now := time.Now()
	ctx := c.Context()

	// Validate user ids up front in one batched query. Only users with an
	// active, non-expired subscription may have traffic recorded against them;
	// unknown or ineligible user ids are silently skipped so one bad entry does
	// not drop the whole batch.
	validUsers := eligibleUserIDs(ctx, req.Traffic, now.Unix())

	// Collect all eligible records and aggregate per-user traffic deltas so the
	// whole batch is written with a single multi-row INSERT and each user's
	// counter is bumped with a single UPDATE.
	var records []model.TrafficRecord
	userDeltas := make(map[uint]int64)
	totalUp, totalDown := int64(0), int64(0)
	for _, entry := range req.Traffic {
		if entry.UserID == 0 || !validUsers[entry.UserID] {
			continue
		}
		if entry.UploadBytes < 0 {
			entry.UploadBytes = 0
		}
		if entry.DownloadBytes < 0 {
			entry.DownloadBytes = 0
		}
		totalUp += entry.UploadBytes
		totalDown += entry.DownloadBytes
		userDeltas[entry.UserID] += entry.UploadBytes + entry.DownloadBytes

		records = append(records, model.TrafficRecord{
			NodeID:        req.NodeID,
			UserID:        entry.UserID,
			UploadBytes:   entry.UploadBytes,
			DownloadBytes: entry.DownloadBytes,
			RecordedAt:    now,
		})
	}

	recorded := 0
	if len(records) > 0 {
		// One multi-row INSERT instead of one INSERT per entry.
		if err := db.DB.WithContext(ctx).Create(&records).Error; err == nil {
			recorded = len(records)
		} else {
			// The whole batch failed to persist; do not credit any user
			// traffic so counters and records cannot diverge.
			userDeltas = nil
		}
	}

	// Accumulate each user's total usage with a single UPDATE per user (only
	// eligible users with a real delta appear in the map).
	for userID, delta := range userDeltas {
		if delta == 0 {
			continue
		}
		db.DB.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).
			Update("traffic_used_bytes", gorm.Expr("traffic_used_bytes + ?", delta))
	}

	// Update node cumulative counters atomically.
	if totalUp > 0 || totalDown > 0 {
		db.DB.WithContext(ctx).Model(&model.Node{}).Where("id = ?", req.NodeID).Updates(map[string]interface{}{
			"traffic_up":   gorm.Expr("traffic_up + ?", totalUp),
			"traffic_down": gorm.Expr("traffic_down + ?", totalDown),
		})
	}

	return c.JSON(fiber.Map{
		"ok":       true,
		"node_id":  req.NodeID,
		"recorded": recorded,
	})
}

// eligibleUserIDs returns the subset of user ids in entries that currently have
// an active, non-expired subscription. The lookup is a single batched query.
func eligibleUserIDs(ctx context.Context, entries []NodeTrafficEntry, nowUnix int64) map[uint]bool {
	eligible := make(map[uint]bool)
	seen := make(map[uint]bool)
	ids := make([]uint, 0, len(entries))
	for _, e := range entries {
		if e.UserID == 0 || seen[e.UserID] {
			continue
		}
		seen[e.UserID] = true
		ids = append(ids, e.UserID)
	}
	if len(ids) == 0 {
		return eligible
	}

	var users []model.User
	if err := db.DB.WithContext(ctx).
		Where("id IN ? AND subscription_status = ? AND (expire_time = 0 OR expire_time > ?)", ids, "active", nowUnix).
		Find(&users).Error; err != nil {
		return eligible
	}
	for _, u := range users {
		eligible[u.ID] = true
	}
	return eligible
}
