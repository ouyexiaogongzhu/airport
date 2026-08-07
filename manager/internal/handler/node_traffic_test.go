package handler

import (
	"bytes"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
)

// createActiveUser inserts a user with an active, unexpired subscription.
func createActiveUser(t *testing.T, username string) *model.User {
	t.Helper()
	user := model.User{
		Username:           username,
		PasswordHash:       "hash",
		Status:             "active",
		ClientToken:        "ct_" + username, // unique index; must be unique per row
		SubscriptionStatus: "active",
		ExpireTime:         time.Now().Add(24 * time.Hour).Unix(),
	}
	if result := db.DB.Create(&user); result.Error != nil {
		t.Fatalf("failed to create user: %v", result.Error)
	}
	return &user
}

func TestReportNodeTraffic_SkipsInvalidUsers(t *testing.T) {
	setupTestDB(t)
	node := createTestNodeInDB(t)
	activeUser := createActiveUser(t, "active_user")

	// Expired subscription must be skipped even though status is "active".
	expiredUser := model.User{
		Username:           "expired_user",
		PasswordHash:       "hash",
		Status:             "active",
		ClientToken:        "ct_expired_user",
		SubscriptionStatus: "active",
		ExpireTime:         time.Now().Add(-1 * time.Hour).Unix(),
	}
	if result := db.DB.Create(&expiredUser); result.Error != nil {
		t.Fatalf("failed to create expired user: %v", result.Error)
	}
	// Non-active subscription must be skipped.
	pendingUser := model.User{
		Username:           "pending_user",
		PasswordHash:       "hash",
		Status:             "active",
		ClientToken:        "ct_pending_user",
		SubscriptionStatus: "pending",
	}
	if result := db.DB.Create(&pendingUser); result.Error != nil {
		t.Fatalf("failed to create pending user: %v", result.Error)
	}

	app := fiber.New()
	app.Post("/node/:token/traffic/report", func(c *fiber.Ctx) error {
		c.Locals("node", node)
		return ReportNodeTraffic(c)
	})

	body := `{"node_id":` + strconv.Itoa(int(node.ID)) + `,"traffic":[` +
		`{"user_id":` + strconv.Itoa(int(activeUser.ID)) + `,"upload_bytes":100,"download_bytes":50},` +
		`{"user_id":99999,"upload_bytes":1000,"download_bytes":1000},` + // non-existent
		`{"user_id":` + strconv.Itoa(int(expiredUser.ID)) + `,"upload_bytes":777,"download_bytes":777},` + // expired
		`{"user_id":` + strconv.Itoa(int(pendingUser.ID)) + `,"upload_bytes":888,"download_bytes":888}` + // pending
		`]}`
	req := httptest.NewRequest("POST", "/node/whatever/traffic/report", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		bodyBytes := make([]byte, 512)
		n, _ := resp.Body.Read(bodyBytes)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes[:n]))
	}

	// Only the active user's usage is accumulated.
	var updated model.User
	if err := db.DB.First(&updated, activeUser.ID).Error; err != nil {
		t.Fatalf("failed to reload active user: %v", err)
	}
	if updated.TrafficUsedBytes != 150 {
		t.Fatalf("expected active user traffic_used_bytes 150, got %d", updated.TrafficUsedBytes)
	}
	for _, u := range []*model.User{&expiredUser, &pendingUser} {
		var reloaded model.User
		if err := db.DB.First(&reloaded, u.ID).Error; err != nil {
			t.Fatalf("failed to reload user: %v", err)
		}
		if reloaded.TrafficUsedBytes != 0 {
			t.Fatalf("expected user %s traffic_used_bytes 0, got %d", u.Username, reloaded.TrafficUsedBytes)
		}
	}

	// Exactly one traffic record is persisted.
	var count int64
	if err := db.DB.Model(&model.TrafficRecord{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count traffic records: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 traffic record, got %d", count)
	}

	// Node cumulative counters only include the valid entry.
	var updatedNode model.Node
	if err := db.DB.First(&updatedNode, node.ID).Error; err != nil {
		t.Fatalf("failed to reload node: %v", err)
	}
	if updatedNode.TrafficUp != 100 || updatedNode.TrafficDown != 50 {
		t.Fatalf("expected node traffic up=100 down=50, got up=%d down=%d", updatedNode.TrafficUp, updatedNode.TrafficDown)
	}
}

func TestReportNodeTraffic_TooManyEntries(t *testing.T) {
	setupTestDB(t)
	node := createTestNodeInDB(t)
	app := fiber.New()
	app.Post("/node/:token/traffic/report", func(c *fiber.Ctx) error {
		c.Locals("node", node)
		return ReportNodeTraffic(c)
	})

	var b strings.Builder
	b.WriteString(`{"node_id":` + strconv.Itoa(int(node.ID)) + `,"traffic":[`)
	for i := 0; i < maxNodeTrafficEntries+1; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"user_id":1,"upload_bytes":1,"download_bytes":1}`)
	}
	b.WriteString(`]}`)

	req := httptest.NewRequest("POST", "/node/whatever/traffic/report", strings.NewReader(b.String()))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}
