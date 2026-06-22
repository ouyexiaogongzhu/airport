package handler

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
)

func setupAdminUser(t *testing.T) *model.User {
	t.Helper()
	user := model.User{
		Username:     "admin",
		PasswordHash: "hash",
		Role:         "admin",
		Status:       "active",
	}
	if result := db.DB.Create(&user); result.Error != nil {
		t.Fatalf("failed to create admin user: %v", result.Error)
	}
	return &user
}

func setupTestAppWithAdminRoutes() *fiber.App {
	app := fiber.New()
	admin := app.Group("/api/v1/admin")
	admin.Post("/nodes", CreateNode)
	admin.Get("/nodes", ListNode)
	admin.Get("/nodes/:id", GetNode)
	admin.Put("/nodes/:id", UpdateNode)
	admin.Delete("/nodes/:id", DeleteNode)
	return app
}

func createTestNode(t *testing.T, app *fiber.App) model.Node {
	t.Helper()
	body := `{"name":"TestNode","type":"v2ray","address":"1.2.3.4","port":443,"protocol":"vmess","user_id":1}`
	req := httptest.NewRequest("POST", "/api/v1/admin/nodes", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("create node request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}
	var node model.Node
	if err := json.NewDecoder(resp.Body).Decode(&node); err != nil {
		t.Fatalf("failed to parse node response: %v", err)
	}
	return node
}

func TestCreateNode_Success(t *testing.T) {
	setupTestDB(t)
	setupAdminUser(t)
	app := setupTestAppWithAdminRoutes()

	body := `{"name":"TestNode","type":"v2ray","address":"1.2.3.4","port":443,"protocol":"vmess","user_id":1}`
	req := httptest.NewRequest("POST", "/api/v1/admin/nodes", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}

	var node model.Node
	if err := json.NewDecoder(resp.Body).Decode(&node); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if node.Name != "TestNode" {
		t.Fatalf("expected node name TestNode, got %s", node.Name)
	}
}

func TestListNode_Empty(t *testing.T) {
	setupTestDB(t)
	setupAdminUser(t)
	app := setupTestAppWithAdminRoutes()

	req := httptest.NewRequest("GET", "/api/v1/admin/nodes", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var nodes []model.Node
	if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("expected empty array, got %d items", len(nodes))
	}
}

func TestCreateAndListNode(t *testing.T) {
	setupTestDB(t)
	setupAdminUser(t)
	app := setupTestAppWithAdminRoutes()

	createTestNode(t, app)

	req := httptest.NewRequest("GET", "/api/v1/admin/nodes", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var nodes []model.Node
	if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
}

func TestGetNode(t *testing.T) {
	setupTestDB(t)
	setupAdminUser(t)
	app := setupTestAppWithAdminRoutes()

	createTestNode(t, app)

	req := httptest.NewRequest("GET", "/api/v1/admin/nodes/1", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var node model.Node
	if err := json.NewDecoder(resp.Body).Decode(&node); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if node.ID != 1 {
		t.Fatalf("expected node ID 1, got %d", node.ID)
	}
}

func TestGetNode_NotFound(t *testing.T) {
	setupTestDB(t)
	setupAdminUser(t)
	app := setupTestAppWithAdminRoutes()

	req := httptest.NewRequest("GET", "/api/v1/admin/nodes/999", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected status 404, got %d", resp.StatusCode)
	}
}

func TestUpdateNode(t *testing.T) {
	setupTestDB(t)
	setupAdminUser(t)
	app := setupTestAppWithAdminRoutes()

	createTestNode(t, app)

	body := `{"status":"active"}`
	req := httptest.NewRequest("PUT", "/api/v1/admin/nodes/1", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var node model.Node
	if err := json.NewDecoder(resp.Body).Decode(&node); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if node.Status != "active" {
		t.Fatalf("expected status active, got %s", node.Status)
	}
}

func TestDeleteNode(t *testing.T) {
	setupTestDB(t)
	setupAdminUser(t)
	app := setupTestAppWithAdminRoutes()

	createTestNode(t, app)

	req := httptest.NewRequest("DELETE", "/api/v1/admin/nodes/1", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("delete request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected delete status 200, got %d", resp.StatusCode)
	}

	req = httptest.NewRequest("GET", "/api/v1/admin/nodes/1", nil)
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("get request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected get status 404, got %d", resp.StatusCode)
	}
}
