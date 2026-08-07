package sync

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/ouyexiaogongzhu/airport/daemon/internal/config"
)

// Syncer handles periodic synchronisation of the node's Xray configuration from
// the Manager API, applies it to the local Xray-core process, and reports
// per-user traffic back to the Manager.
type Syncer struct {
	cfg    *config.Config
	client *http.Client
	stopCh chan struct{}

	mu           sync.Mutex
	lastConfig   map[string]interface{}
	lastSyncTime time.Time
	lastError    error
	lastName     string
	// lastAppliedConfigHash is the hash of the last config applied to xray.
	lastAppliedHash string
	// running tracks whether the managed xray process is considered up.
	running bool
	// xrayCmd is the currently managed xray process, if any.
	xrayCmd *exec.Cmd
	// lastTraffic is the last cumulative traffic snapshot per user id.
	lastTraffic map[uint]TrafficDelta
}

// NodeConfig is the node metadata derived from the last synced Xray config.
type NodeConfig struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Address     string `json:"address"`
	Port        int    `json:"port"`
	Protocol    string `json:"protocol"`
	Status      string `json:"status"`
	TrafficUp   int64  `json:"traffic_up"`
	TrafficDown int64  `json:"traffic_down"`
	UserID      uint   `json:"user_id"`
	Users       int    `json:"users"`
}

// TrafficDelta is a per-user cumulative traffic snapshot.
type TrafficDelta struct {
	Upload   int64
	Download int64
}

// SyncResult holds the result of a sync operation.
type SyncResult struct {
	Success     bool   `json:"success"`
	NodeCount   int    `json:"node_count"`
	Message     string `json:"message"`
	SyncedAt    string `json:"synced_at"`
	ConfigVersion int64 `json:"config_version"`
}

// NewSyncer creates a new Syncer.
func NewSyncer(cfg *config.Config) *Syncer {
	return &Syncer{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		stopCh:      make(chan struct{}),
		lastTraffic: make(map[uint]TrafficDelta),
	}
}

// Start begins the periodic sync loop. Runs until Stop() is called.
func (s *Syncer) Start() {
	log.Printf("[sync] starting sync loop (interval=%s)", s.cfg.SyncInterval)

	// Do an initial sync immediately
	if err := s.Sync(); err != nil {
		log.Printf("[sync] initial sync failed: %v", err)
	}

	ticker := time.NewTicker(s.cfg.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.Sync(); err != nil {
				log.Printf("[sync] sync failed: %v", err)
			}
		case <-s.stopCh:
			log.Printf("[sync] sync loop stopped")
			return
		}
	}
}

// Stop signals the sync loop to stop.
func (s *Syncer) Stop() {
	close(s.stopCh)
}

// Sync performs a single sync: fetch config, apply to xray, report traffic.
func (s *Syncer) Sync() error {
	cfg, err := s.fetchConfig()
	if err != nil {
		s.mu.Lock()
		s.lastError = err
		s.lastSyncTime = time.Now()
		s.mu.Unlock()
		return fmt.Errorf("fetch config: %w", err)
	}

	if err := s.applyConfig(cfg.Config); err != nil {
		s.mu.Lock()
		s.lastError = err
		s.lastSyncTime = time.Now()
		s.mu.Unlock()
		return fmt.Errorf("apply config: %w", err)
	}

	if err := s.reportTraffic(cfg.NodeID); err != nil {
		log.Printf("[sync] traffic report failed (non-fatal): %v", err)
	}

	s.mu.Lock()
	s.lastConfig = cfg.Config
	s.lastName = cfg.Name
	s.lastSyncTime = time.Now()
	s.lastError = nil
	s.mu.Unlock()

	log.Printf("[sync] synced node config %q (node_id=%d)", cfg.Name, cfg.NodeID)
	return nil
}

// nodeConfigResponse mirrors the Manager's /node/:token/config response.
type nodeConfigResponse struct {
	NodeID   uint                   `json:"node_id"`
	Name     string                 `json:"name"`
	Protocol string                 `json:"protocol"`
	Config   map[string]interface{} `json:"config"`
}

// fetchConfig calls the Manager API to get this node's Xray config.
func (s *Syncer) fetchConfig() (*nodeConfigResponse, error) {
	url := fmt.Sprintf("%s/api/v1/node/%s/config", s.cfg.ManagerURL, s.cfg.ManagerToken)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	s.signRequest(req, nil)
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("manager returned %d: %s", resp.StatusCode, string(body))
	}

	var cfg nodeConfigResponse
	if err := json.Unmarshal(body, &cfg); err != nil {
		return nil, fmt.Errorf("parse response: %w (body: %s)", err, string(body))
	}
	return &cfg, nil
}

// signRequest adds the X-Node-Timestamp / X-Node-Signature HMAC headers the
// manager requires. The signature binds method, path, timestamp and body, so
// it also serves as a request-integrity check for traffic reports.
func (s *Syncer) signRequest(req *http.Request, body []byte) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, nodeHMACSecret(s.cfg.ManagerToken))
	mac.Write([]byte(req.Method))
	mac.Write([]byte("\n"))
	mac.Write([]byte(req.URL.Path))
	mac.Write([]byte("\n"))
	mac.Write([]byte(ts))
	mac.Write([]byte("\n"))
	mac.Write(body)
	req.Header.Set("X-Node-Timestamp", ts)
	req.Header.Set("X-Node-Signature", hex.EncodeToString(mac.Sum(nil)))
}

// nodeHMACSecret mirrors the manager's derivation: sha256("rfplay-node-hmac-v1:" + token).
func nodeHMACSecret(token string) []byte {
	h := sha256.Sum256([]byte("rfplay-node-hmac-v1:" + token))
	return h[:]
}

// applyConfig writes the Xray config to disk and reloads the local Xray process.
// If the config hash is unchanged, no restart is triggered.
func (s *Syncer) applyConfig(cfg map[string]interface{}) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// Determine if the config changed since last apply.
	hash := configHash(data)

	s.mu.Lock()
	changed := hash != s.lastAppliedHash
	s.mu.Unlock()
	if !changed {
		return nil
	}

	// Write config to disk.
	if err := os.MkdirAll(s.cfg.DataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	configPath := filepath.Join(s.cfg.DataDir, "xray.json")
	// 0600: the config embeds every active subscriber's proxy credentials.
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	log.Printf("[sync] config changed, reloading xray (%d bytes)", len(data))

	// Reload: if the daemon is configured with an xray binary path, restart it.
	if s.cfg.XrayBinary != "" {
		if err := s.restartXray(configPath); err != nil {
			log.Printf("[sync] xray restart failed (continuing): %v", err)
		} else {
			s.mu.Lock()
			s.running = true
			s.mu.Unlock()
		}
	} else {
		log.Printf("[sync] no xray binary configured; config written to %s", configPath)
	}

	s.mu.Lock()
	s.lastAppliedHash = hash
	s.mu.Unlock()
	return nil
}

// restartXray stops any running xray process and starts a new one with the
// freshly-written config.
func (s *Syncer) restartXray(configPath string) error {
	// Stop existing process.
	s.stopXray()
	time.Sleep(500 * time.Millisecond)

	cmd := exec.Command(s.cfg.XrayBinary, "-c", configPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start xray: %w", err)
	}

	s.mu.Lock()
	s.xrayCmd = cmd
	s.mu.Unlock()
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("[sync] xray process exited: %v", err)
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
		}
	}()
	return nil
}

// stopXray terminates the managed xray process, if any.
func (s *Syncer) stopXray() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.xrayCmd != nil && s.xrayCmd.Process != nil {
		_ = s.xrayCmd.Process.Kill()
		s.xrayCmd = nil
	}
}

// reportTraffic reads the node's own traffic accounting and reports deltas to
// the Manager. The daemon reads cumulative totals from a stats file that the
// xray process writes (see deploy docs); if unavailable, it reports nothing.
func (s *Syncer) reportTraffic(nodeID uint) error {
	stats, err := s.readTrafficStats()
	if err != nil || len(stats) == 0 {
		return nil // no traffic accounting yet — nothing to report
	}

	// Compute deltas from the last snapshot.
	type reportEntry struct {
		UserID       uint  `json:"user_id"`
		UploadBytes  int64 `json:"upload_bytes"`
		DownloadBytes int64 `json:"download_bytes"`
	}
	var entries []reportEntry
	s.mu.Lock()
	for userID, cur := range stats {
		prev, ok := s.lastTraffic[userID]
		if !ok {
			// First snapshot: establish the baseline without reporting.
			s.lastTraffic[userID] = cur
			continue
		}
		up := cur.Upload
		down := cur.Download
		if cur.Upload < prev.Upload {
			up = cur.Upload // counter reset
		} else {
			up = cur.Upload - prev.Upload
		}
		if cur.Download < prev.Download {
			down = cur.Download
		} else {
			down = cur.Download - prev.Download
		}
		if up > 0 || down > 0 {
			entries = append(entries, reportEntry{UserID: userID, UploadBytes: up, DownloadBytes: down})
		}
		s.lastTraffic[userID] = cur
	}
	s.mu.Unlock()

	if len(entries) == 0 {
		return nil
	}

	payload := map[string]interface{}{
		"node_id": nodeID,
		"traffic": entries,
	}
	body, _ := json.Marshal(payload)

	url := fmt.Sprintf("%s/api/v1/node/%s/traffic/report", s.cfg.ManagerURL, s.cfg.ManagerToken)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create report request: %w", err)
	}
	s.signRequest(req, body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("report request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("manager report returned %d: %s", resp.StatusCode, string(respBody))
	}

	log.Printf("[sync] reported traffic for %d user(s)", len(entries))
	return nil
}

// readTrafficStats reads cumulative per-user traffic from the xray stats file.
// The file is written by the xray process or a companion exporter; its format
// is {"user_id": {"upload": N, "download": N}, ...}.
func (s *Syncer) readTrafficStats() (map[uint]TrafficDelta, error) {
	path := filepath.Join(s.cfg.DataDir, "traffic_stats.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw map[string]struct {
		Upload   int64 `json:"upload"`
		Download int64 `json:"download"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	out := make(map[uint]TrafficDelta, len(raw))
	for k, v := range raw {
		var id uint
		if _, err := fmt.Sscanf(k, "%d", &id); err != nil {
			continue
		}
		out[id] = TrafficDelta{Upload: v.Upload, Download: v.Download}
	}
	return out, nil
}

// configHash returns a quick content hash for change detection.
func configHash(data []byte) string {
	var h uint64 = 14695981039346656037
	for _, b := range data {
		h ^= uint64(b)
		h *= 1099511628211
	}
	return fmt.Sprintf("%x", h)
}

// LastSyncResult returns the last sync result from in-memory state.
func (s *Syncer) LastSyncResult() (*SyncResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lastConfig == nil && s.lastError == nil {
		return &SyncResult{
			Success:  false,
			Message:  "no config synced yet",
			SyncedAt: time.Now().Format(time.RFC3339),
		}, nil
	}
	if s.lastError != nil {
		return &SyncResult{
			Success:  false,
			Message:  s.lastError.Error(),
			SyncedAt: s.lastSyncTime.Format(time.RFC3339),
		}, nil
	}

	version := int64(0)
	if meta, ok := s.lastConfig["_meta"].(map[string]interface{}); ok {
		if v, ok := meta["version"].(float64); ok {
			version = int64(v)
		}
	}

	return &SyncResult{
		Success:       true,
		NodeCount:     1,
		Message:       "ok",
		SyncedAt:      s.lastSyncTime.Format(time.RFC3339),
		ConfigVersion: version,
	}, nil
}

// GetLocalNodes returns node metadata derived from the last synced config.
func (s *Syncer) GetLocalNodes() ([]NodeConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lastConfig == nil {
		return nil, os.ErrNotExist
	}
	meta, _ := s.lastConfig["_meta"].(map[string]interface{})
	nodeID := uint(0)
	if id, ok := meta["node_id"].(float64); ok {
		nodeID = uint(id)
	}
	userIDs, _ := meta["user_ids"].([]interface{})

	nodes := make([]NodeConfig, 0, 1)
	nodes = append(nodes, NodeConfig{
		ID:     nodeID,
		Name:   s.lastName,
		Status: "active",
		Users:  len(userIDs),
	})
	return nodes, nil
}

// GetDataDirForTesting returns the syncer's data directory — used in tests.
func (s *Syncer) GetDataDirForTesting() string {
	return s.cfg.DataDir
}

// GetConfigForTesting returns the syncer's config — used in tests.
func (s *Syncer) GetConfigForTesting() *config.Config {
	return s.cfg
}
