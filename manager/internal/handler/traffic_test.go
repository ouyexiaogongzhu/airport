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

func setupTestAppWithTrafficRoutes() *fiber.App {
	app := fiber.New()
	admin := app.Group("/api/v1/admin")
	admin.Post("/traffic/report", ReportTraffic)
	admin.Get("/traffic/stats", GetTrafficStats)
	return app
}

func createTestNodeInDB(t *testing.T) *model.Node {
	t.Helper()
	node := model.Node{
		Name:     "TrafficNode",
		Type:     "v2ray",
		Address:  "1.2.3.4",
		Port:     443,
		Protocol: "vmess",
		Status:   "active",
		UserID:   1,
	}
	if result := db.DB.Create(&node); result.Error != nil {
		t.Fatalf("failed to create node: %v", result.Error)
	}
	return &node
}

func TestReportTraffic(t *testing.T) {
	setupTestDB(t)
	setupAdminUser(t)
	createTestNodeInDB(t)
	app := setupTestAppWithTrafficRoutes()

	body := `{"node_id":1,"user_id":1,"upload_bytes":1000,"download_bytes":2000}`
	req := httptest.NewRequest("POST", "/api/v1/admin/traffic/report", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}

	var record model.TrafficRecord
	if err := json.NewDecoder(resp.Body).Decode(&record); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if record.UploadBytes != 1000 {
		t.Fatalf("expected upload_bytes 1000, got %d", record.UploadBytes)
	}
	if record.DownloadBytes != 2000 {
		t.Fatalf("expected download_bytes 2000, got %d", record.DownloadBytes)
	}
}

func TestGetTrafficStats(t *testing.T) {
	setupTestDB(t)
	setupAdminUser(t)
	createTestNodeInDB(t)
	app := setupTestAppWithTrafficRoutes()

	reportBody := `{"node_id":1,"user_id":1,"upload_bytes":1000,"download_bytes":2000}`
	req := httptest.NewRequest("POST", "/api/v1/admin/traffic/report", bytes.NewReader([]byte(reportBody)))
	req.Header.Set("Content-Type", "application/json")
	if _, err := app.Test(req, -1); err != nil {
		t.Fatalf("report request failed: %v", err)
	}

	req = httptest.NewRequest("GET", "/api/v1/admin/traffic/stats", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("stats request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var result struct {
		Data []TrafficSummary `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("expected 1 stats item, got %d", len(result.Data))
	}
	if result.Data[0].TotalUpload != 1000 {
		t.Fatalf("expected total_upload 1000, got %d", result.Data[0].TotalUpload)
	}
	if result.Data[0].TotalDownload != 2000 {
		t.Fatalf("expected total_download 2000, got %d", result.Data[0].TotalDownload)
	}
}
