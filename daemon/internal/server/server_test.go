package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ouyexiaogongzhu/airport/daemon/internal/config"
	"github.com/ouyexiaogongzhu/airport/daemon/internal/sync"
)

func setupTestServer(t *testing.T) (*Server, *sync.Syncer) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.ManagerToken = "test-token"
	cfg.DataDir = t.TempDir()
	syncer := sync.NewSyncer(cfg)
	srv := New(cfg, syncer)
	return srv, syncer
}

// performRequest helper sends a GET to the given path on srv and returns the
// response.
func performRequest(srv *Server, path string) *http.Response {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	resp, _ := srv.App().Test(req, -1) // -1 = no timeout
	return resp
}

// setupSyncedServer runs a Sync against a mock manager before returning the
// server, so the in-memory node metadata is populated.
func setupSyncedServer(t *testing.T) (*Server, *sync.Syncer) {
	t.Helper()
	srv, syncer := setupTestServer(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/node/test-token/config":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(sampleConfigJSON))
		case "/api/v1/node/test-token/traffic/report":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(ts.Close)

	syncer.GetConfigForTesting().ManagerURL = ts.URL
	if err := syncer.Sync(); err != nil {
		t.Fatalf("syncer.Sync() setup error: %v", err)
	}
	return srv, syncer
}

const sampleConfigJSON = `{
  "node_id": 1,
  "name": "node-x",
  "protocol": "vless",
  "config": {
    "log": {"loglevel": "info"},
    "inbounds": [{"port": 443, "protocol": "vless", "settings": {"clients": [{"id": "u1"}]}}],
    "_meta": {"node_id": 1, "user_ids": [1], "version": 7}
  }
}`

func TestHealthEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDir = t.TempDir()
	srv := New(cfg, sync.NewSyncer(cfg))

	resp := performRequest(srv, "/health")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", body["status"])
	}
	if body["service"] != "daemon-api" {
		t.Errorf("expected service=daemon-api, got %v", body["service"])
	}
	if _, ok := body["timestamp"]; !ok {
		t.Error("expected timestamp field")
	}
}

func TestStatusEndpoint(t *testing.T) {
	srv, _ := setupSyncedServer(t)

	resp := performRequest(srv, "/api/v1/status")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// node_id should be 1 from DefaultConfig
	if body["node_id"] != float64(1) {
		t.Errorf("expected node_id=1, got %v", body["node_id"])
	}

	lastSync, ok := body["last_sync"].(map[string]interface{})
	if !ok {
		t.Fatal("expected last_sync object")
	}
	if lastSync["success"] != true {
		t.Errorf("expected last_sync.success=true, got %v", lastSync["success"])
	}
}

func TestNodesEndpoint_NoData(t *testing.T) {
	srv, _ := setupTestServer(t)

	resp := performRequest(srv, "/api/v1/nodes")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "not_found" {
		t.Errorf("expected error=not_found, got %v", body["error"])
	}
}

func TestNodesEndpoint_WithData(t *testing.T) {
	srv, _ := setupSyncedServer(t)

	resp := performRequest(srv, "/api/v1/nodes")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("expected 1 node, got %d", len(body))
	}
	if body[0]["name"] != "node-x" {
		t.Errorf("expected name=node-x, got %v", body[0]["name"])
	}
}

func TestTrafficEndpoint(t *testing.T) {
	srv, syncer := setupTestServer(t)

	// Populate node metadata via a manager sync (traffic counters are 0 until
	// the first report, so node_count is the meaningful assertion here).
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/node/test-token/config":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(sampleConfigJSON))
		case "/api/v1/node/test-token/traffic/report":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()
	syncer.GetConfigForTesting().ManagerURL = ts.URL
	if err := syncer.Sync(); err != nil {
		t.Fatalf("syncer.Sync() setup error: %v", err)
	}

	resp := performRequest(srv, "/api/v1/traffic")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body["total_up"] != float64(0) {
		t.Errorf("expected total_up=0 before traffic report, got %v", body["total_up"])
	}
	if body["total_down"] != float64(0) {
		t.Errorf("expected total_down=0 before traffic report, got %v", body["total_down"])
	}
	if body["node_count"] != float64(1) {
		t.Errorf("expected node_count=1, got %v", body["node_count"])
	}
}
