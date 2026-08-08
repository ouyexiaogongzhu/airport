package sync

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ouyexiaogongzhu/airport/daemon/internal/config"
)

func setupTestSyncer(t testing.TB, mockServerURL string) *Syncer {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.ManagerURL = mockServerURL
	cfg.ManagerToken = "test-token"
	cfg.DataDir = t.TempDir()
	return NewSyncer(cfg)
}

func sampleConfig() map[string]interface{} {
	return map[string]interface{}{
		"log": map[string]interface{}{"loglevel": "info"},
		"inbounds": []interface{}{
			map[string]interface{}{
				"port":     443,
				"protocol": "vless",
				"settings": map[string]interface{}{
					"clients": []interface{}{
						map[string]interface{}{"id": "uuid-1", "flow": "xtls-rprx-vision"},
					},
				},
			},
		},
		"_meta": map[string]interface{}{
			"node_id":  1,
			"user_ids": []interface{}{float64(1)},
			"version":  float64(100),
		},
	}
}

func TestFetchConfig_Success(t *testing.T) {
	sample := nodeConfigResponse{
		NodeID:   1,
		Name:     "node-1",
		Protocol: "vless",
		Config:   sampleConfig(),
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/node/test-token/config" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sample)
	}))
	defer ts.Close()

	syncer := setupTestSyncer(t, ts.URL)
	cfg, err := syncer.fetchConfig()
	if err != nil {
		t.Fatalf("fetchConfig returned error: %v", err)
	}
	if cfg.NodeID != 1 || cfg.Name != "node-1" {
		t.Errorf("unexpected config metadata: %+v", cfg)
	}
	inbounds, ok := cfg.Config["inbounds"].([]interface{})
	if !ok || len(inbounds) != 1 {
		t.Fatalf("expected 1 inbound, got %v", cfg.Config["inbounds"])
	}
}

func TestFetchConfig_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	syncer := setupTestSyncer(t, ts.URL)
	_, err := syncer.fetchConfig()
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestFetchConfig_BadJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{not valid json}"))
	}))
	defer ts.Close()

	syncer := setupTestSyncer(t, ts.URL)
	_, err := syncer.fetchConfig()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestApplyConfig_WritesXrayJSON(t *testing.T) {
	syncer := setupTestSyncer(t, "http://localhost:9999")

	if err := syncer.applyConfig(sampleConfig()); err != nil {
		t.Fatalf("applyConfig error: %v", err)
	}

	// Verify file was written
	path := filepath.Join(syncer.cfg.DataDir, "xray.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	// Config embeds subscriber credentials; must not be world-readable.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat written file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("expected config file mode 0600, got %v", perm)
	}

	var loaded map[string]interface{}
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("written file is not valid JSON: %v", err)
	}
	if _, ok := loaded["inbounds"]; !ok {
		t.Errorf("expected inbounds in written config")
	}
}

func TestApplyConfig_Idempotent(t *testing.T) {
	syncer := setupTestSyncer(t, "http://localhost:9999")

	if err := syncer.applyConfig(sampleConfig()); err != nil {
		t.Fatalf("first applyConfig error: %v", err)
	}
	// Second apply with the same config should not rewrite (hash unchanged).
	if err := syncer.applyConfig(sampleConfig()); err != nil {
		t.Fatalf("second applyConfig error: %v", err)
	}
}

func TestApplyConfig_SkipsRewriteWhenVersionUnchanged(t *testing.T) {
	syncer := setupTestSyncer(t, "http://localhost:9999")

	if err := syncer.applyConfig(sampleConfig()); err != nil {
		t.Fatalf("first applyConfig error: %v", err)
	}
	path := filepath.Join(syncer.cfg.DataDir, "xray.json")
	first, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after first apply: %v", err)
	}

	// A rewrite would visibly change the mtime (macOS APFS has ns resolution),
	// so give the second apply a window in which to differ.
	time.Sleep(10 * time.Millisecond)

	if err := syncer.applyConfig(sampleConfig()); err != nil {
		t.Fatalf("second applyConfig error: %v", err)
	}
	second, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after second apply: %v", err)
	}
	if !second.ModTime().Equal(first.ModTime()) {
		t.Errorf("config file was rewritten on identical version: mtime %v -> %v", first.ModTime(), second.ModTime())
	}
}

func TestApplyConfig_WritesWhenVersionChanges(t *testing.T) {
	syncer := setupTestSyncer(t, "http://localhost:9999")

	cfg := sampleConfig()
	if err := syncer.applyConfig(cfg); err != nil {
		t.Fatalf("first applyConfig error: %v", err)
	}

	// Bump _meta.version: the fast path must not trigger and the file is
	// rewritten with the new content.
	bumped := sampleConfig()
	bumped["_meta"] = map[string]interface{}{
		"node_id":  1,
		"user_ids": []interface{}{float64(1), float64(2)},
		"version":  float64(101),
	}
	if err := syncer.applyConfig(bumped); err != nil {
		t.Fatalf("second applyConfig error: %v", err)
	}

	path := filepath.Join(syncer.cfg.DataDir, "xray.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	var loaded map[string]interface{}
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("written file is not valid JSON: %v", err)
	}
	meta, _ := loaded["_meta"].(map[string]interface{})
	if v, ok := meta["version"].(float64); !ok || int64(v) != 101 {
		t.Errorf("expected version 101 in rewritten file, got %v", meta["version"])
	}

	// And a third apply with the same bumped config must skip the rewrite.
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat before third apply: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := syncer.applyConfig(bumped); err != nil {
		t.Fatalf("third applyConfig error: %v", err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after third apply: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("config file was rewritten on unchanged version: mtime %v -> %v", before.ModTime(), after.ModTime())
	}
}

func TestSync_Integration(t *testing.T) {
	sample := nodeConfigResponse{
		NodeID:   1,
		Name:     "alpha",
		Protocol: "vless",
		Config:   sampleConfig(),
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/node/test-token/config":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(sample)
		case "/api/v1/node/test-token/traffic/report":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	syncer := setupTestSyncer(t, ts.URL)

	if err := syncer.Sync(); err != nil {
		t.Fatalf("Sync() returned error: %v", err)
	}

	nodes, err := syncer.GetLocalNodes()
	if err != nil {
		t.Fatalf("GetLocalNodes error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Name != "alpha" {
		t.Errorf("unexpected node: %+v", nodes[0])
	}
	if nodes[0].Users != 1 {
		t.Errorf("expected 1 active user, got %d", nodes[0].Users)
	}
}

func TestLastSyncResult(t *testing.T) {
	sample := nodeConfigResponse{
		NodeID:   1,
		Name:     "gamma",
		Protocol: "vless",
		Config:   sampleConfig(),
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sample)
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
	if result.ConfigVersion != 100 {
		t.Errorf("expected ConfigVersion=100, got %d", result.ConfigVersion)
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

func BenchmarkConfigHash(b *testing.B) {
	data, err := json.MarshalIndent(sampleConfig(), "", "  ")
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		configHash(data)
	}
}

// BenchmarkApplyConfig_MarshalHash measures the per-tick work the old
// applyConfig did even when the config was unchanged: marshal + hash.
func BenchmarkApplyConfig_MarshalHash(b *testing.B) {
	cfg := sampleConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			b.Fatal(err)
		}
		configHash(data)
	}
}

// BenchmarkApplyConfig_Unchanged measures applyConfig on an unchanged config:
// the _meta.version fast path skips marshal and write entirely.
func BenchmarkApplyConfig_Unchanged(b *testing.B) {
	syncer := setupTestSyncer(b, "http://localhost:9999")
	cfg := sampleConfig()
	if err := syncer.applyConfig(cfg); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := syncer.applyConfig(cfg); err != nil {
			b.Fatal(err)
		}
	}
}

func TestReportTraffic_Delta(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/node/test-token/traffic/report" {
			json.NewDecoder(r.Body).Decode(&gotBody)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"ok":true}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	syncer := setupTestSyncer(t, ts.URL)

	// First snapshot establishes the baseline; no report expected.
	statsFile := filepath.Join(syncer.cfg.DataDir, "traffic_stats.json")
	writeStats := func(upload, download int64) {
		data, _ := json.Marshal(map[string]interface{}{
			"1": map[string]int64{"upload": upload, "download": download},
		})
		os.WriteFile(statsFile, data, 0644)
	}

	writeStats(100, 200)
	if err := syncer.reportTraffic(1); err != nil {
		t.Fatalf("reportTraffic error: %v", err)
	}
	if gotBody != nil {
		t.Fatal("expected no report on first snapshot (baseline only)")
	}

	// Second snapshot: delta of 50 up / 30 down should be reported.
	gotBody = nil
	writeStats(150, 230)
	if err := syncer.reportTraffic(1); err != nil {
		t.Fatalf("reportTraffic error: %v", err)
	}
	if gotBody == nil {
		t.Fatal("expected a report on second snapshot")
	}
	nodeID := int(gotBody["node_id"].(float64))
	if nodeID != 1 {
		t.Fatalf("expected node_id 1, got %d", nodeID)
	}
	traffic := gotBody["traffic"].([]interface{})
	if len(traffic) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(traffic))
	}
	entry := traffic[0].(map[string]interface{})
	if entry["upload_bytes"].(float64) != 50 {
		t.Errorf("expected upload delta 50, got %v", entry["upload_bytes"])
	}
	if entry["download_bytes"].(float64) != 30 {
		t.Errorf("expected download delta 30, got %v", entry["download_bytes"])
	}
}
