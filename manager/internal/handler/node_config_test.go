package handler

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
	"gorm.io/gorm"
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

// countUserQueries installs a GORM query callback that counts SELECTs touching
// the users table, so a test can prove a cache hit did not re-load user rows.
func countUserQueries(t *testing.T) *atomic.Int64 {
	t.Helper()
	var n atomic.Int64
	err := db.DB.Callback().Query().After("gorm:query").
		Register("count_user_queries", func(tx *gorm.DB) {
			if strings.Contains(tx.Statement.SQL.String(), "FROM `users`") {
				n.Add(1)
			}
		})
	if err != nil {
		t.Fatalf("failed to register query counter: %v", err)
	}
	t.Cleanup(func() {
		db.DB.Callback().Query().Remove("count_user_queries")
	})
	return &n
}

// decodeConfig marshals and unmarshals a built config so nested values use
// plain Go types ([]interface{}, map[string]interface{}) regardless of the
// fiber.Map type aliases used internally.
func decodeConfig(t *testing.T, cfg map[string]interface{}) map[string]interface{} {
	t.Helper()
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}
	return out
}

func TestBuildNodeXrayConfig_CacheHitSkipsFullRebuild(t *testing.T) {
	setupTestDB(t)
	ResetNodeConfigCache()
	createActiveTestUser(t)
	createActiveTestUser(t)
	node := createTestNodeInDB(t)

	queries := countUserQueries(t)

	cfg1, err := BuildNodeXrayConfig(node, db.DB)
	if err != nil {
		t.Fatalf("first build failed: %v", err)
	}
	firstQueries := queries.Load()

	queries.Store(0)
	cfg2, err := BuildNodeXrayConfig(node, db.DB)
	if err != nil {
		t.Fatalf("second build failed: %v", err)
	}
	secondQueries := queries.Load()

	// The first call runs the id fingerprint plus the full user-row load; a
	// cache hit must skip the full load entirely, leaving only the cheap
	// fingerprint query.
	if secondQueries >= firstQueries {
		t.Fatalf("expected cache hit to avoid the full user load: first call %d user queries, second %d", firstQueries, secondQueries)
	}
	// The served config must be byte-identical across the cache hit.
	b1, err1 := json.Marshal(cfg1)
	b2, err2 := json.Marshal(cfg2)
	if err1 != nil || err2 != nil {
		t.Fatalf("failed to marshal configs: %v %v", err1, err2)
	}
	if string(b1) != string(b2) {
		t.Fatal("expected identical config across a cache hit, got different JSON")
	}
}

func TestBuildNodeXrayConfig_CacheInvalidatedOnUserChange(t *testing.T) {
	setupTestDB(t)
	ResetNodeConfigCache()
	keep := createActiveTestUser(t)
	drop := createActiveTestUser(t)
	node := createTestNodeInDB(t)

	cfg1, err := BuildNodeXrayConfig(node, db.DB)
	if err != nil {
		t.Fatalf("first build failed: %v", err)
	}

	// Two active users before the change.
	decoded := decodeConfig(t, cfg1)
	inbounds := decoded["inbounds"].([]interface{})
	clients := inbounds[0].(map[string]interface{})["settings"].(map[string]interface{})["clients"].([]interface{})
	if len(clients) != 2 {
		t.Fatalf("expected 2 clients initially, got %d", len(clients))
	}

	// Deactivating a user changes the id-set version, so the cached config
	// must be rebuilt without that user.
	if err := db.DB.Model(drop).Update("subscription_status", "pending").Error; err != nil {
		t.Fatalf("failed to deactivate user: %v", err)
	}
	cfg2, err := BuildNodeXrayConfig(node, db.DB)
	if err != nil {
		t.Fatalf("rebuild failed: %v", err)
	}
	decoded = decodeConfig(t, cfg2)
	inbounds = decoded["inbounds"].([]interface{})
	clients = inbounds[0].(map[string]interface{})["settings"].(map[string]interface{})["clients"].([]interface{})
	if len(clients) != 1 {
		t.Fatalf("expected 1 client after deactivating a user, got %d", len(clients))
	}
	if client := clients[0].(map[string]interface{}); client["id"] != keep.VlessUUID {
		t.Fatalf("expected remaining client uuid %s, got %v", keep.VlessUUID, client["id"])
	}
}

func TestBuildNodeXrayConfig_CacheInvalidatedOnNodeChange(t *testing.T) {
	setupTestDB(t)
	ResetNodeConfigCache()
	createActiveTestUser(t)
	node := createTestNodeInDB(t)

	cfg1, err := BuildNodeXrayConfig(node, db.DB)
	if err != nil {
		t.Fatalf("first build failed: %v", err)
	}
	decoded := decodeConfig(t, cfg1)
	inbounds := decoded["inbounds"].([]interface{})
	if port := inbounds[0].(map[string]interface{})["port"]; port != float64(443) {
		t.Fatalf("expected initial port 443, got %v", port)
	}

	// A node edit (port change) bumps node.UpdatedAt, which must invalidate
	// the cached config even though the user set is unchanged.
	if err := db.DB.Model(&model.Node{}).Where("id = ?", node.ID).Update("port", 8443).Error; err != nil {
		t.Fatalf("failed to update node: %v", err)
	}
	if err := db.DB.First(node, node.ID).Error; err != nil {
		t.Fatalf("failed to reload node: %v", err)
	}
	cfg2, err := BuildNodeXrayConfig(node, db.DB)
	if err != nil {
		t.Fatalf("rebuild failed: %v", err)
	}
	decoded = decodeConfig(t, cfg2)
	inbounds = decoded["inbounds"].([]interface{})
	if port := inbounds[0].(map[string]interface{})["port"]; port != float64(8443) {
		t.Fatalf("expected rebuilt port 8443, got %v", port)
	}
}

func TestGetNodeConfigForDaemon_ConfigCacheSurvivesHeartbeat(t *testing.T) {
	setupTestDB(t)
	ResetNodeConfigCache()
	createActiveTestUser(t)
	node := createTestNodeInDB(t)

	app := fiber.New()
	app.Get("/node/:token/config", func(c *fiber.Ctx) error {
		c.Locals("node", node)
		return GetNodeConfigForDaemon(c)
	})

	queries := countUserQueries(t)

	if resp, err := app.Test(httptest.NewRequest("GET", "/node/x/config", nil), -1); err != nil || resp.StatusCode != fiber.StatusOK {
		t.Fatalf("first config pull failed: err=%v status=%d", err, resp.StatusCode)
	}
	queries.Store(0)

	// The daemon pulls again; the handler touches last_heartbeat (via
	// UpdateColumn, so updated_at is untouched) and the cached config must be
	// served without re-loading any user rows.
	if resp, err := app.Test(httptest.NewRequest("GET", "/node/x/config", nil), -1); err != nil || resp.StatusCode != fiber.StatusOK {
		t.Fatalf("second config pull failed: err=%v status=%d", err, resp.StatusCode)
	}
	if got := queries.Load(); got != 1 {
		t.Fatalf("expected 1 fingerprint query on the cached pull, got %d (heartbeat must not invalidate the config cache)", got)
	}
}

func TestBuildNodeXrayConfig_HeartbeatTouchPreservesFingerprint(t *testing.T) {
	setupTestDB(t)
	ResetNodeConfigCache()
	createActiveTestUser(t)
	node := createTestNodeInDB(t)

	queries := countUserQueries(t)

	if _, err := BuildNodeXrayConfig(node, db.DB); err != nil {
		t.Fatalf("first build failed: %v", err)
	}
	queries.Store(0)

	// Mirror the daemon-handler flow: touch last_heartbeat then reload the node
	// as the middleware's token-cache refresh would on a later request. With
	// UpdateColumn the fingerprint (node.UpdatedAt) is unchanged and the next
	// build is a cache hit.
	now := time.Now()
	if err := db.DB.Model(&model.Node{}).Where("id = ?", node.ID).UpdateColumn("last_heartbeat", &now).Error; err != nil {
		t.Fatalf("failed to touch heartbeat: %v", err)
	}
	if err := db.DB.First(node, node.ID).Error; err != nil {
		t.Fatalf("failed to reload node: %v", err)
	}
	if _, err := BuildNodeXrayConfig(node, db.DB); err != nil {
		t.Fatalf("second build failed: %v", err)
	}
	if got := queries.Load(); got != 1 {
		t.Fatalf("expected 1 fingerprint query after a heartbeat touch, got %d", got)
	}
}

func TestUserSetVersion_OrderDependentButDeterministic(t *testing.T) {
	// userSetVersion is an order-dependent FNV-style hash. BuildNodeXrayConfig
	// sorts the id set before hashing so the same set always yields the same
	// version; unsorted input is a caller error.
	a := userSetVersion([]uint{3, 1, 2})
	b := userSetVersion([]uint{1, 2, 3})
	if a == b {
		t.Fatal("expected an order-dependent version; callers must sort input first")
	}
	// Hashing the same sorted slice twice is deterministic.
	if b != userSetVersion([]uint{1, 2, 3}) {
		t.Fatal("expected deterministic version for identical sorted input")
	}
	// A different id set must produce a different version.
	if c := userSetVersion([]uint{1, 2, 4}); c == b {
		t.Fatal("expected different version for a different id set")
	}
}
