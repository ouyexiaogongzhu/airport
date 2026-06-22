package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// writeNodesJSON writes nodes directly to the data dir for testing.
func writeNodesJSON(syncer *sync.Syncer, nodes []sync.NodeConfig) error {
	data, err := json.MarshalIndent(nodes, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(syncer.GetDataDirForTesting(), "nodes.json")
	return os.WriteFile(path, data, 0644)
}

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
	srv, syncer := setupTestServer(t)

	// Pre-run sync so we have data
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	syncer.GetConfigForTesting().ManagerURL = ts.URL
	if err := syncer.Sync(); err != nil {
		t.Fatalf("syncer.Sync() setup error: %v", err)
	}

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
	srv, syncer := setupTestServer(t)

	// Pre-write nodes.json to simulate synced data
	nodes := []sync.NodeConfig{
		{ID: 1, Name: "node-x", Address: "10.0.0.1", Port: 8080, Status: "online", TrafficUp: 100, TrafficDown: 200, Protocol: "tcp", Type: "relay"},
		{ID: 2, Name: "node-y", Address: "10.0.0.2", Port: 8081, Status: "offline", TrafficUp: 300, TrafficDown: 400, Protocol: "udp", Type: "relay"},
	}
	if err := writeNodesJSON(syncer, nodes); err != nil {
		t.Fatalf("write test nodes error: %v", err)
	}

	resp := performRequest(srv, "/api/v1/nodes")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(body))
	}
	if body[0]["name"] != "node-x" {
		t.Errorf("expected name=node-x, got %v", body[0]["name"])
	}
}

func TestTrafficEndpoint(t *testing.T) {
	srv, syncer := setupTestServer(t)

	// Pre-write nodes with traffic data
	nodes := []sync.NodeConfig{
		{ID: 1, Name: "a", Address: "1", Port: 80, Status: "up", TrafficUp: 1000, TrafficDown: 2000, Protocol: "tcp", Type: "relay"},
		{ID: 2, Name: "b", Address: "2", Port: 81, Status: "up", TrafficUp: 3000, TrafficDown: 4000, Protocol: "tcp", Type: "relay"},
		{ID: 3, Name: "c", Address: "3", Port: 82, Status: "down", TrafficUp: 500, TrafficDown: 600, Protocol: "udp", Type: "relay"},
	}
	if err := writeNodesJSON(syncer, nodes); err != nil {
		t.Fatalf("write test nodes error: %v", err)
	}

	resp := performRequest(srv, "/api/v1/traffic")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// total_up: 1000+3000+500 = 4500, total_down: 2000+4000+600 = 6600
	if body["total_up"] != float64(4500) {
		t.Errorf("expected total_up=4500, got %v", body["total_up"])
	}
	if body["total_down"] != float64(6600) {
		t.Errorf("expected total_down=6600, got %v", body["total_down"])
	}
	if body["node_count"] != float64(3) {
		t.Errorf("expected node_count=3, got %v", body["node_count"])
	}
}
