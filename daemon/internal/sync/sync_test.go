package sync

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ouyexiaogongzhu/airport/daemon/internal/config"
)

func setupTestSyncer(t *testing.T, mockServerURL string) *Syncer {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.ManagerURL = mockServerURL
	cfg.ManagerToken = "test-token"
	cfg.DataDir = t.TempDir()
	return NewSyncer(cfg)
}

func TestFetchNodes_Success(t *testing.T) {
	sampleNodes := []NodeConfig{
		{ID: 1, Name: "node-1", Address: "10.0.0.1", Port: 8080, Status: "online", TrafficUp: 100, TrafficDown: 200, Protocol: "tcp", Type: "relay"},
		{ID: 2, Name: "node-2", Address: "10.0.0.2", Port: 8081, Status: "offline", TrafficUp: 300, TrafficDown: 400, Protocol: "udp", Type: "relay"},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sampleNodes)
	}))
	defer ts.Close()

	syncer := setupTestSyncer(t, ts.URL)
	nodes, err := syncer.fetchNodes()
	if err != nil {
		t.Fatalf("fetchNodes returned error: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if nodes[0].ID != 1 || nodes[0].Name != "node-1" {
		t.Errorf("unexpected node[0]: %+v", nodes[0])
	}
	if nodes[1].ID != 2 || nodes[1].Name != "node-2" {
		t.Errorf("unexpected node[1]: %+v", nodes[1])
	}
}

func TestFetchNodes_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	syncer := setupTestSyncer(t, ts.URL)
	_, err := syncer.fetchNodes()
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestFetchNodes_BadJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{not valid json}"))
	}))
	defer ts.Close()

	syncer := setupTestSyncer(t, ts.URL)
	_, err := syncer.fetchNodes()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestWriteLocalConfig(t *testing.T) {
	syncer := setupTestSyncer(t, "http://localhost:9999")

	nodes := []NodeConfig{
		{ID: 10, Name: "test-node", Address: "192.168.1.10", Port: 9090, Status: "active", TrafficUp: 500, TrafficDown: 600, Protocol: "tcp", Type: "relay"},
	}

	if err := syncer.writeLocalConfig(nodes); err != nil {
		t.Fatalf("writeLocalConfig error: %v", err)
	}

	// Verify file was written
	path := filepath.Join(syncer.cfg.DataDir, "nodes.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	var loaded []NodeConfig
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("written file is not valid JSON: %v", err)
	}
	if len(loaded) != 1 || loaded[0].ID != 10 {
		t.Errorf("unexpected data in written file: %+v", loaded)
	}
}

func TestSync_Integration(t *testing.T) {
	sampleNodes := []NodeConfig{
		{ID: 1, Name: "alpha", Address: "10.0.0.1", Port: 8080, Status: "online", TrafficUp: 111, TrafficDown: 222, Protocol: "tcp", Type: "relay"},
		{ID: 2, Name: "beta", Address: "10.0.0.2", Port: 8081, Status: "online", TrafficUp: 333, TrafficDown: 444, Protocol: "udp", Type: "relay"},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sampleNodes)
	}))
	defer ts.Close()

	syncer := setupTestSyncer(t, ts.URL)

	if err := syncer.Sync(); err != nil {
		t.Fatalf("Sync() returned error: %v", err)
	}

	// Verify nodes are readable via GetLocalNodes
	nodes, err := syncer.GetLocalNodes()
	if err != nil {
		t.Fatalf("GetLocalNodes error: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if nodes[0].Name != "alpha" || nodes[1].Name != "beta" {
		t.Errorf("unexpected nodes: %+v", nodes)
	}
}

func TestLastSyncResult(t *testing.T) {
	sampleNodes := []NodeConfig{
		{ID: 1, Name: "gamma", Address: "10.0.0.3", Port: 8080, Status: "online", TrafficUp: 10, TrafficDown: 20, Protocol: "tcp", Type: "relay"},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sampleNodes)
	}))
	defer ts.Close()

	syncer := setupTestSyncer(t, ts.URL)

	if err := syncer.Sync(); err != nil {
		t.Fatalf("Sync() error: %v", err)
	}

	result, err := syncer.LastSyncResult()
	if err != nil {
		t.Fatalf("LastSyncResult error: %v", err)
	}
	if !result.Success {
		t.Error("expected Success=true after sync")
	}
	if result.NodeCount != 1 {
		t.Errorf("expected NodeCount=1, got %d", result.NodeCount)
	}
	if result.SyncedAt == "" {
		t.Error("expected non-empty SyncedAt")
	}
}

func TestLastSyncResult_NoData(t *testing.T) {
	// Create a syncer but never call Sync()
	syncer := setupTestSyncer(t, "http://localhost:9999")

	result, err := syncer.LastSyncResult()
	if err != nil {
		t.Fatalf("LastSyncResult should not error when no data: %v", err)
	}
	if result.Success {
		t.Error("expected Success=false when no local config")
	}
	if result.NodeCount != 0 {
		t.Errorf("expected NodeCount=0, got %d", result.NodeCount)
	}
}
