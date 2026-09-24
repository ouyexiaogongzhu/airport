package sync

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ouyexiaogongzhu/airport/gateway/internal/config"
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

// fakeStats returns a queryStats replacement that yields each queued result
// once (xray counters are reset on read), then nothing.
func fakeStats(queue ...map[uint]TrafficDelta) func() (map[uint]TrafficDelta, error) {
	return func() (map[uint]TrafficDelta, error) {
		if len(queue) == 0 {
			return nil, nil
		}
		next := queue[0]
		queue = queue[1:]
		return next, nil
	}
}

func TestReportTraffic_ReportsResetCounters(t *testing.T) {
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
	syncer.nodeID = 7
	syncer.queryStats = fakeStats(map[uint]TrafficDelta{1: {Upload: 50, Download: 30}})

	if err := syncer.reportTraffic(); err != nil {
		t.Fatalf("reportTraffic error: %v", err)
	}
	if gotBody == nil {
		t.Fatal("expected a report")
	}
	if nodeID := int(gotBody["node_id"].(float64)); nodeID != 7 {
		t.Fatalf("expected node_id 7, got %d", nodeID)
	}
	batchID, _ := gotBody["batch_id"].(string)
	if len(batchID) != 32 { // 16 random bytes hex
		t.Fatalf("expected a 32-char batch_id in payload, got %q", gotBody["batch_id"])
	}
	traffic := gotBody["traffic"].([]interface{})
	if len(traffic) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(traffic))
	}
	entry := traffic[0].(map[string]interface{})
	if entry["user_id"].(float64) != 1 || entry["upload_bytes"].(float64) != 50 || entry["download_bytes"].(float64) != 30 {
		t.Errorf("unexpected entry %v", entry)
	}
	if len(syncer.pending) != 0 {
		t.Errorf("expected pending cleared after successful report, got %v", syncer.pending)
	}

	// Nothing new: no report.
	gotBody = nil
	if err := syncer.reportTraffic(); err != nil {
		t.Fatalf("reportTraffic error: %v", err)
	}
	if gotBody != nil {
		t.Errorf("expected no report without new traffic, got %v", gotBody)
	}
}

func TestReportTraffic_KeepsPendingOnFailure(t *testing.T) {
	fail := true
	var bodies []map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]interface{}
		json.NewDecoder(r.Body).Decode(&b)
		bodies = append(bodies, b)
		if fail {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	syncer := setupTestSyncer(t, ts.URL)
	syncer.nodeID = 7
	syncer.queryStats = fakeStats(
		map[uint]TrafficDelta{1: {Upload: 100, Download: 200}},
		map[uint]TrafficDelta{1: {Upload: 1, Download: 2}, 2: {Upload: 0, Download: 5}},
	)

	if err := syncer.reportTraffic(); err == nil {
		t.Fatal("expected error from failing manager")
	}
	if got := syncer.pending[1]; got.Upload != 100 || got.Download != 200 {
		t.Fatalf("expected traffic kept pending after failure, got %+v", got)
	}
	if syncer.outstandingBatch == nil {
		t.Fatal("expected an outstanding batch after failed report")
	}
	if _, err := os.Stat(filepath.Join(syncer.cfg.DataDir, "pending.json")); err != nil {
		t.Fatalf("outstanding batch must be persisted as soon as it is created: %v", err)
	}

	fail = false
	if err := syncer.reportTraffic(); err != nil {
		t.Fatalf("reportTraffic error: %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("expected two report attempts, got %d", len(bodies))
	}
	// The retry must be identical (same batch_id, same entries) so the
	// worker's dedup table hits — NOT merged with the newly collected traffic.
	if bodies[0]["batch_id"] != bodies[1]["batch_id"] {
		t.Fatalf("expected identical batch_id on retry, got %v then %v", bodies[0]["batch_id"], bodies[1]["batch_id"])
	}
	firstJSON, _ := json.Marshal(bodies[0])
	secondJSON, _ := json.Marshal(bodies[1])
	if string(firstJSON) != string(secondJSON) {
		t.Errorf("expected identical retry body, got %s then %s", firstJSON, secondJSON)
	}
	if syncer.outstandingBatch != nil {
		t.Errorf("expected outstanding batch cleared after 200")
	}
	// Traffic collected while the batch was in flight stays pending.
	if got := syncer.pending[1]; got.Upload != 1 || got.Download != 2 {
		t.Errorf("expected newly collected traffic kept pending, got %+v", syncer.pending[1])
	}
	if got := syncer.pending[2]; got.Upload != 0 || got.Download != 5 {
		t.Errorf("expected user 2 kept pending, got %+v", syncer.pending[2])
	}
}

func TestSync_ReportsTrafficWhenConfigFetchFails(t *testing.T) {
	reported := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/node/test-token/traffic/report":
			reported = true
			w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer ts.Close()

	syncer := setupTestSyncer(t, ts.URL)
	syncer.queryStats = fakeStats(map[uint]TrafficDelta{3: {Upload: 1, Download: 1}})
	if err := syncer.Sync(); err == nil {
		t.Fatal("expected Sync error when config fetch fails")
	}
	if !reported {
		t.Error("expected traffic to be reported even though config fetch failed")
	}
}

func TestSync_TakesNodeIDFromResponse(t *testing.T) {
	sample := nodeConfigResponse{NodeID: 42, Name: "n", Protocol: "vless", Config: sampleConfig()}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sample)
	}))
	defer ts.Close()

	syncer := setupTestSyncer(t, ts.URL)
	if err := syncer.Sync(); err != nil {
		t.Fatalf("Sync() error: %v", err)
	}
	if syncer.NodeID() != 42 {
		t.Errorf("expected node id 42 from config response, got %d", syncer.NodeID())
	}
}

func TestParseStatsQuery(t *testing.T) {
	out := []byte(`{
  "stat": [
    {"name": "user>>>u12>>>traffic>>>uplink", "value": "1024"},
    {"name": "user>>>u12>>>traffic>>>downlink", "value": 4096},
    {"name": "user>>>u3>>>traffic>>>downlink"},
    {"name": "inbound>>>api>>>traffic>>>uplink", "value": "9"},
    {"name": "user>>>alice>>>traffic>>>uplink", "value": "9"}
  ]
}`)
	stats, err := parseStatsQuery(out)
	if err != nil {
		t.Fatalf("parseStatsQuery error: %v", err)
	}
	if got := stats[12]; got.Upload != 1024 || got.Download != 4096 {
		t.Errorf("unexpected stats for user 12: %+v", got)
	}
	if got := stats[3]; got.Upload != 0 || got.Download != 0 {
		t.Errorf("expected zero for omitted value, got %+v", got)
	}
	if len(stats) != 2 {
		t.Errorf("expected 2 users, got %v", stats)
	}

	empty, err := parseStatsQuery([]byte("  \n"))
	if err != nil || len(empty) != 0 {
		t.Errorf("expected empty result for empty output, got %v, %v", empty, err)
	}
	if _, err := parseStatsQuery([]byte("not json")); err == nil {
		t.Error("expected error for invalid output")
	}
}

func TestMetaAPIPort(t *testing.T) {
	if p := metaAPIPort(map[string]interface{}{"_meta": map[string]interface{}{"api_port": float64(10086)}}); p != 10086 {
		t.Errorf("expected 10086, got %d", p)
	}
	if p := metaAPIPort(sampleConfig()); p != defaultAPIPort {
		t.Errorf("expected default port, got %d", p)
	}
}

func TestStop_Idempotent(t *testing.T) {
	syncer := setupTestSyncer(t, "http://localhost:9999")
	syncer.Stop()
	syncer.Stop()
}

// TestReportTraffic_ChunksOver4000: more than 4000 pending users must not hit
// the worker's per-report limit; the surplus stays pending for a next batch.
func TestReportTraffic_ChunksOver4000(t *testing.T) {
	var gotCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]interface{}
		json.NewDecoder(r.Body).Decode(&b)
		gotCount = len(b["traffic"].([]interface{}))
		w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	syncer := setupTestSyncer(t, ts.URL)
	stats := make(map[uint]TrafficDelta, 4001)
	for i := uint(1); i <= 4001; i++ {
		stats[i] = TrafficDelta{Upload: 1}
	}
	syncer.queryStats = fakeStats(stats)

	if err := syncer.reportTraffic(); err != nil {
		t.Fatalf("reportTraffic error: %v", err)
	}
	if gotCount != 4000 {
		t.Errorf("expected the batch capped at 4000 entries, got %d", gotCount)
	}
	if len(syncer.pending) != 1 {
		t.Errorf("expected 1 user left pending for the next batch, got %d", len(syncer.pending))
	}

	// Next round: the remainder goes out in a fresh batch.
	gotCount = 0
	if err := syncer.reportTraffic(); err != nil {
		t.Fatalf("reportTraffic error: %v", err)
	}
	if gotCount != 1 || len(syncer.pending) != 0 {
		t.Errorf("expected the remainder reported and pending cleared, got %d entries, pending %v", gotCount, syncer.pending)
	}
}

// TestLoadPending_LegacyAndOutstanding: the old bare-map pending.json still
// loads, and a persisted in-flight batch is restored so a crash cannot turn a
// resent batch into a double count.
func TestLoadPending_LegacyAndOutstanding(t *testing.T) {
	syncer := setupTestSyncer(t, "http://localhost:9999")

	// Old format: bare map of deltas.
	if err := os.WriteFile(filepath.Join(syncer.cfg.DataDir, "pending.json"),
		[]byte(`{"1":{"Upload":100,"Download":200}}`), 0600); err != nil {
		t.Fatal(err)
	}
	syncer.loadPending()
	if got := syncer.pending[1]; got.Upload != 100 || got.Download != 200 {
		t.Fatalf("legacy pending not restored: %+v", syncer.pending)
	}
	if syncer.outstandingBatch != nil {
		t.Errorf("unexpected outstanding batch from legacy file")
	}

	// New format with an in-flight batch.
	syncer2 := setupTestSyncer(t, "http://localhost:9999")
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	sent := &outstandingReport{BatchID: "abcdef1234567890", Entries: []reportEntry{{UserID: 1, UploadBytes: 7, DownloadBytes: 8}}}
	syncer2.mu.Lock()
	syncer2.pending[1] = TrafficDelta{Upload: 7, Download: 8}
	syncer2.outstandingBatch = sent
	syncer2.persistPendingLocked()
	syncer2.mu.Unlock()

	// A fresh process reloads from the same dir and must resend the same batch.
	syncer3 := setupTestSyncer(t, ts.URL)
	syncer3.cfg.DataDir = syncer2.cfg.DataDir
	syncer3.loadPending()
	if err := syncer3.reportTraffic(); err != nil {
		t.Fatalf("reportTraffic error: %v", err)
	}
	if gotBody["batch_id"] != sent.BatchID {
		t.Errorf("expected restored batch %q to be resent, got %v", sent.BatchID, gotBody["batch_id"])
	}
	if len(syncer3.pending) != 0 || syncer3.outstandingBatch != nil {
		t.Errorf("expected pending and outstanding cleared after 200, got %v / %v", syncer3.pending, syncer3.outstandingBatch)
	}
}
