package handler

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
)

func TestGetNodeConfigForDaemon_InactiveNode(t *testing.T) {
	setupTestDB(t)
	node := createTestNodeInDB(t)
	if err := db.DB.Model(node).Update("status", "inactive").Error; err != nil {
		t.Fatalf("failed to set node inactive: %v", err)
	}

	app := fiber.New()
	app.Get("/node/:token/config", func(c *fiber.Ctx) error {
		c.Locals("node", node)
		return GetNodeConfigForDaemon(c)
	})

	req := httptest.NewRequest("GET", "/node/whatever/config", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if payload.Error != "NODE_DISABLED" {
		t.Fatalf("expected NODE_DISABLED, got %q", payload.Error)
	}
}

func TestGenerateNodeConfig_InactiveNode(t *testing.T) {
	setupTestDB(t)
	node := createTestNodeInDB(t)
	if err := db.DB.Model(node).Update("status", "inactive").Error; err != nil {
		t.Fatalf("failed to set node inactive: %v", err)
	}

	app := fiber.New()
	app.Get("/api/v1/admin/nodes/:id/config", GenerateNodeConfig)

	req := httptest.NewRequest("GET", "/api/v1/admin/nodes/"+strconv.Itoa(int(node.ID))+"/config", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if payload.Error != "NODE_DISABLED" {
		t.Fatalf("expected NODE_DISABLED, got %q", payload.Error)
	}
}

func TestGenerateNodeConfig_ActiveNode(t *testing.T) {
	setupTestDB(t)
	node := createTestNodeInDB(t)
	createActiveUser(t, "config_user")

	app := fiber.New()
	app.Get("/api/v1/admin/nodes/:id/config", GenerateNodeConfig)

	req := httptest.NewRequest("GET", "/api/v1/admin/nodes/"+strconv.Itoa(int(node.ID))+"/config", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	// GenerateNodeConfig returns the xray config document directly.
	meta, ok := payload["_meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected _meta in config, got %v", payload["_meta"])
	}
	userIDs, ok := meta["user_ids"].([]interface{})
	if !ok || len(userIDs) != 1 {
		t.Fatalf("expected 1 user id in _meta, got %v", meta["user_ids"])
	}
}
