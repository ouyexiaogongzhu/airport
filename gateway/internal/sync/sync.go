package sync

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ouyexiaogongzhu/airport/gateway/internal/config"
)

// defaultAPIPort is the Xray StatsService port used when the manager's config
// does not carry _meta.api_port.
const defaultAPIPort = 10085

// Syncer handles periodic synchronisation of the node's Xray configuration from
// the Manager API, applies it to the local Xray-core process (and keeps that
// process running), and reports per-user traffic back to the Manager.
type Syncer struct {
	cfg      *config.Config
	client   *http.Client
	stopCh   chan struct{}
	stopOnce sync.Once
	// loopDone is closed when the Start loop has returned; Stop waits for it
	// (bounded) so an in-flight sync — whose traffic counters were already
	// reset on read — is not killed mid-report.
	loopDone chan struct{}

	// xrayMu serialises starting/stopping the xray process.
	xrayMu sync.Mutex

	mu           sync.Mutex
	lastConfig   map[string]interface{}
	lastSyncTime time.Time
	lastError    error
	lastName     string
	// lastAppliedConfigHash is the hash of the last config applied to xray.
	lastAppliedHash string
	// lastAppliedVersion is the _meta.version of the last config applied to
	// xray; combined with lastAppliedHash it lets applyConfig skip marshalling
	// and writing when nothing has changed.
	lastAppliedVersion int64
	// running tracks whether the managed xray process is considered up.
	running bool
	// stopping is set by Stop so crashed xray is not relaunched.
	stopping bool
	// xrayCmd is the currently managed xray process, if any; xrayDone is
	// closed once that process has exited.
	xrayCmd  *exec.Cmd
	xrayDone chan struct{}
	// nodeID is taken from the manager's config response.
	nodeID uint
	// apiPort is the StatsService port of the running xray config.
	apiPort int
	// pending holds traffic read from xray (counters are reset on read) that
	// the manager has not acknowledged yet; it is merged into the next report.
	pending map[uint]TrafficDelta
	// outstandingBatch is a batch that was sent but never acknowledged (non-200
	// or network error). It is resent byte-for-byte — same batch_id, same
	// entries — until accepted, so the worker's dedup table recognises the
	// replay instead of double-counting.
	outstandingBatch *outstandingReport
	// queryStats reads and resets per-user counters; replaceable in tests.
	queryStats func() (map[uint]TrafficDelta, error)
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

// TrafficDelta is per-user traffic in bytes.
type TrafficDelta struct {
	Upload   int64
	Download int64
}

// SyncResult holds the result of a sync operation.
type SyncResult struct {
	Success       bool   `json:"success"`
	NodeCount     int    `json:"node_count"`
	Message       string `json:"message"`
	SyncedAt      string `json:"synced_at"`
	ConfigVersion int64  `json:"config_version"`
}

// NewSyncer creates a new Syncer.
func NewSyncer(cfg *config.Config) *Syncer {
	s := &Syncer{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		stopCh:   make(chan struct{}),
		loopDone: make(chan struct{}),
		pending:  make(map[uint]TrafficDelta),
	}
	s.queryStats = s.queryXrayStats
	return s
}

// Start begins the periodic sync loop. Runs until Stop() is called.
func (s *Syncer) Start() {
	defer close(s.loopDone)
	log.Printf("[sync] starting sync loop (interval=%s)", s.cfg.SyncInterval)
	if s.cfg.XrayBinary == "" {
		// A typo in xray_binary would otherwise look perfectly healthy while
		// the node serves nothing.
		log.Printf("[sync] WARNING: xray_binary not configured; config will be written but xray will not run")
	}

	// Restore traffic that was read (counters reset) but not yet acknowledged.
	s.loadPending()

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

// Stop signals the sync loop to stop and terminates the managed xray process.
// Safe to call more than once.
func (s *Syncer) Stop() {
	s.stopOnce.Do(func() {
		s.mu.Lock()
		s.stopping = true
		s.mu.Unlock()
		close(s.stopCh)

		// Wait (bounded) for an in-flight sync — its counters were already
		// reset on read, killing it here would lose that traffic.
		select {
		case <-s.loopDone:
		case <-time.After(35 * time.Second):
			log.Printf("[sync] timed out waiting for sync loop; continuing shutdown")
		}
		// Final flush: collect whatever xray counted since the last sync and
		// report it before the process goes away.
		if err := s.reportTraffic(); err != nil {
			log.Printf("[sync] final traffic report failed (pending kept on disk): %v", err)
		}

		s.xrayMu.Lock()
		s.stopXrayLocked()
		s.xrayMu.Unlock()
	})
}

// NodeID returns the node id reported by the manager (0 before the first
// successful config pull).
func (s *Syncer) NodeID() uint {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nodeID
}

// Sync performs a single sync: fetch config, apply to xray, report traffic.
// Traffic is reported even when fetching or applying the config fails, so
// usage keeps being accounted for while the manager is unreachable.
func (s *Syncer) Sync() error {
	var syncErr error
	cfg, err := s.fetchConfig()
	if err != nil {
		syncErr = fmt.Errorf("fetch config: %w", err)
	} else {
		s.mu.Lock()
		s.nodeID = cfg.NodeID
		s.mu.Unlock()
		if err := s.applyConfig(cfg.Config); err != nil {
			syncErr = fmt.Errorf("apply config: %w", err)
		}
	}

	if err := s.reportTraffic(); err != nil {
		log.Printf("[sync] traffic report failed (kept for next report): %v", err)
	}

	s.mu.Lock()
	s.lastSyncTime = time.Now()
	s.lastError = syncErr
	if syncErr == nil {
		s.lastConfig = cfg.Config
		s.lastName = cfg.Name
	}
	s.mu.Unlock()

	if syncErr != nil {
		return syncErr
	}
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
	if cfg.Config == nil {
		return nil, fmt.Errorf("manager response has no config (body: %s)", string(body))
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
// If the config is unchanged, no write or restart is triggered. When the
// restart fails the config is not recorded as applied, so the next sync
// retries it.
func (s *Syncer) applyConfig(cfg map[string]interface{}) error {
	// Fast path: _meta.version is a stable fingerprint derived by the manager
	// from the user set and node transport, so an equal version implies
	// identical content. When it matches the last applied version we can skip
	// marshalling the whole config and writing it again.
	version, hasVersion := configVersion(cfg)
	if hasVersion {
		s.mu.Lock()
		unchanged := version == s.lastAppliedVersion && s.lastAppliedHash != ""
		s.mu.Unlock()
		if unchanged {
			return nil
		}
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// Determine if the config changed since last apply.
	hash := configHash(data)

	s.mu.Lock()
	changed := hash != s.lastAppliedHash
	if !changed && hasVersion {
		// Content identical (e.g. a version bump without content change):
		// remember the version so the fast path works on subsequent ticks.
		s.lastAppliedVersion = version
	}
	s.mu.Unlock()
	if !changed {
		return nil
	}

	// Write config to disk atomically: temp file → `xray run -test` on the
	// temp file → rename. A rejected config must never overwrite the last
	// known-good xray.json on disk, or the next crash-loop relaunch would
	// repeatedly fail on it.
	if err := os.MkdirAll(s.cfg.DataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	configPath := filepath.Join(s.cfg.DataDir, "xray.json")
	// 0600: the config embeds every active subscriber's proxy credentials.
	// Suffix must end in ".json": Xray detects format from the filename and
	// rejects bare ".tmp" with "Failed to get format".
	tmpPath := configPath + ".tmp.json"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	log.Printf("[sync] config changed, reloading xray (%d bytes)", len(data))

	if s.cfg.XrayBinary != "" {
		if out, err := exec.Command(s.cfg.XrayBinary, "run", "-test", "-c", tmpPath).CombinedOutput(); err != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("xray rejected config (keeping current process and on-disk config): %v: %s", err, strings.TrimSpace(string(out)))
		}
		// Restarting resets xray's counters: collect them first.
		s.collectTraffic()
		if err := s.restartXray(tmpPath, configPath); err != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("restart xray: %w", err)
		}
	} else {
		if err := os.Rename(tmpPath, configPath); err != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("install config: %w", err)
		}
		log.Printf("[sync] no xray binary configured; config written to %s", configPath)
	}

	s.mu.Lock()
	s.lastAppliedHash = hash
	if hasVersion {
		s.lastAppliedVersion = version
	}
	s.apiPort = metaAPIPort(cfg)
	s.mu.Unlock()
	return nil
}

// configVersion extracts the stable _meta.version from a config map. The
// version is derived by the manager from the user set and node transport, so
// it is unchanged whenever the config content is unchanged. It returns
// ok=false when the config carries no version; callers then fall back to
// content hashing.
func configVersion(cfg map[string]interface{}) (int64, bool) {
	meta, ok := cfg["_meta"].(map[string]interface{})
	if !ok {
		return 0, false
	}
	switch v := meta["version"].(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case json.Number:
		n, err := v.Int64()
		return n, err == nil
	}
	return 0, false
}

// metaAPIPort extracts _meta.api_port (the StatsService port), falling back
// to defaultAPIPort.
func metaAPIPort(cfg map[string]interface{}) int {
	if meta, ok := cfg["_meta"].(map[string]interface{}); ok {
		if p, ok := meta["api_port"].(float64); ok && p > 0 && p < 65536 {
			return int(p)
		}
	}
	return defaultAPIPort
}

// restartXray stops any running xray process, moves the tested temp config
// into place and starts a new one with it.
func (s *Syncer) restartXray(tmpPath, configPath string) error {
	s.xrayMu.Lock()
	defer s.xrayMu.Unlock()
	s.stopXrayLocked()
	if err := os.Rename(tmpPath, configPath); err != nil {
		return fmt.Errorf("install config: %w", err)
	}
	return s.startXrayLocked(configPath)
}

// startXrayLocked launches xray and a watcher that relaunches it if it exits
// unexpectedly. Caller must hold xrayMu.
func (s *Syncer) startXrayLocked(configPath string) error {
	s.mu.Lock()
	stopping := s.stopping
	s.mu.Unlock()
	if stopping {
		return fmt.Errorf("syncer is stopping")
	}

	if s.orphanXrayLikely() {
		return fmt.Errorf(
			"nothing managed but 127.0.0.1:%d already answers: an orphan xray from a previous gateway is likely still serving (possibly stale config); kill it (pkill -f 'xray run') or reboot before syncing",
			s.statsPort(),
		)
	}

	cmd := exec.Command(s.cfg.XrayBinary, "run", "-c", configPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start xray: %w", err)
	}
	done := make(chan struct{})
	go s.watchXray(cmd, done, configPath)

	// `xray run -test` does not bind ports: catch configs that die instantly
	// (port already taken, bad runtime state) instead of reporting success and
	// letting the watcher enter a crash loop.
	select {
	case <-done:
		return fmt.Errorf("xray exited immediately after start (port conflict or runtime error); config kept on disk")
	case <-time.After(1500 * time.Millisecond):
	}

	s.mu.Lock()
	s.xrayCmd = cmd
	s.xrayDone = done
	s.running = true
	s.mu.Unlock()

	return nil
}

// orphanXrayLikely reports whether the StatsService port answers while we are
// not managing any xray process — the signature of an orphan left behind when
// a previous gateway was SIGKILLed (its cleanup never ran).
func (s *Syncer) orphanXrayLikely() bool {
	s.mu.Lock()
	managed := s.xrayCmd != nil
	s.mu.Unlock()
	if managed {
		return false
	}
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(s.statsPort()), 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// statsPort returns the StatsService port of the last applied config.
func (s *Syncer) statsPort() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.apiPort == 0 {
		return defaultAPIPort
	}
	return s.apiPort
}

// watchXray waits for the xray process to exit. If it was not stopped on
// purpose (restart or Stop), it is relaunched with exponential backoff.
func (s *Syncer) watchXray(cmd *exec.Cmd, done chan struct{}, configPath string) {
	err := cmd.Wait()
	close(done)

	s.mu.Lock()
	current := s.xrayCmd == cmd
	if current {
		s.xrayCmd = nil
		s.xrayDone = nil
		s.running = false
	}
	unexpected := current && !s.stopping
	s.mu.Unlock()
	if !unexpected {
		return
	}

	log.Printf("[sync] xray exited unexpectedly: %v; relaunching", err)
	backoff := 2 * time.Second
	for {
		select {
		case <-s.stopCh:
			return
		case <-time.After(backoff):
		}

		s.xrayMu.Lock()
		s.mu.Lock()
		skip := s.xrayCmd != nil || s.stopping
		s.mu.Unlock()
		if skip {
			// Already restarted by a sync, or shutting down.
			s.xrayMu.Unlock()
			return
		}
		startErr := s.startXrayLocked(configPath)
		s.xrayMu.Unlock()
		if startErr == nil {
			log.Printf("[sync] xray relaunched")
			return
		}
		log.Printf("[sync] xray relaunch failed: %v", startErr)
		if backoff < time.Minute {
			backoff *= 2
		}
	}
}

// stopXrayLocked terminates the managed xray process, if any, and waits for
// it to exit so its ports are free. Caller must hold xrayMu.
func (s *Syncer) stopXrayLocked() {
	s.mu.Lock()
	cmd, done := s.xrayCmd, s.xrayDone
	s.xrayCmd = nil
	s.xrayDone = nil
	s.running = false
	s.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}

	_ = cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-done:
		return
	case <-time.After(5 * time.Second):
	}
	_ = cmd.Process.Kill()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		log.Printf("[sync] xray (pid %d) did not exit after kill", cmd.Process.Pid)
	}
}

// reportEntry is one user's traffic in a report.
type reportEntry struct {
	UserID        uint  `json:"user_id"`
	UploadBytes   int64 `json:"upload_bytes"`
	DownloadBytes int64 `json:"download_bytes"`
}

// maxBatchEntries caps one report batch below the worker's 5000-entry limit;
// anything more stays pending and is sent in a following batch.
const maxBatchEntries = 4000

// outstandingReport is one report batch: an id the worker deduplicates on plus
// the exact entries it covers.
type outstandingReport struct {
	BatchID string        `json:"batch_id"`
	Entries []reportEntry `json:"entries"`
}

// reportPayload is the traffic report request body.
type reportPayload struct {
	NodeID  uint          `json:"node_id"`
	BatchID string        `json:"batch_id"`
	Traffic []reportEntry `json:"traffic"`
}

// newBatchID returns 16 random bytes hex-encoded.
func newBatchID() string {
	b := make([]byte, 16)
	if _, err := crand.Read(b); err != nil {
		// crypto/rand only fails when the OS entropy source is broken; a
		// time-derived id still keeps the protocol working.
		return fmt.Sprintf("t%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// reportTraffic reads per-user traffic from xray (reset on read), adds it to
// the pending totals and reports them as a batch identified by batch_id. An
// unacknowledged batch is resent unchanged (same batch_id, same entries) so
// the worker's dedup table recognises the replay; pending totals are only
// cleared after the manager acknowledges the report.
func (s *Syncer) reportTraffic() error {
	s.collectTraffic()

	s.mu.Lock()
	var batch *outstandingReport
	if s.outstandingBatch != nil {
		// Previous attempt never got a 200: resend it verbatim.
		batch = s.outstandingBatch
	} else {
		entries := make([]reportEntry, 0, len(s.pending))
		for userID, d := range s.pending {
			if d.Upload > 0 || d.Download > 0 {
				entries = append(entries, reportEntry{UserID: userID, UploadBytes: d.Upload, DownloadBytes: d.Download})
			}
		}
		if len(entries) > maxBatchEntries {
			entries = entries[:maxBatchEntries]
		}
		if len(entries) > 0 {
			batch = &outstandingReport{BatchID: newBatchID(), Entries: entries}
			s.outstandingBatch = batch
			// Persist now: if the request lands but the reply is lost, the
			// batch must survive a crash to be resent for dedup.
			s.persistPendingLocked()
		}
	}
	nodeID := s.nodeID
	s.mu.Unlock()

	if batch == nil {
		return nil
	}

	body, _ := json.Marshal(reportPayload{NodeID: nodeID, BatchID: batch.BatchID, Traffic: batch.Entries})

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

	// Accepted: subtract this batch from pending and forget it; anything
	// collected meanwhile stays pending for the next batch.
	s.mu.Lock()
	for _, e := range batch.Entries {
		d := s.pending[e.UserID]
		d.Upload -= e.UploadBytes
		d.Download -= e.DownloadBytes
		if d.Upload <= 0 && d.Download <= 0 {
			delete(s.pending, e.UserID)
		} else {
			s.pending[e.UserID] = d
		}
	}
	s.outstandingBatch = nil
	s.persistPendingLocked()
	s.mu.Unlock()

	log.Printf("[sync] reported traffic batch %s for %d user(s)", batch.BatchID, len(batch.Entries))
	return nil
}

// collectTraffic moves xray's per-user counters into the pending totals.
func (s *Syncer) collectTraffic() {
	stats, err := s.queryStats()
	if err != nil {
		log.Printf("[sync] read xray stats: %v", err)
		return
	}
	if len(stats) == 0 {
		return
	}
	s.mu.Lock()
	for userID, d := range stats {
		p := s.pending[userID]
		p.Upload += d.Upload
		p.Download += d.Download
		s.pending[userID] = p
	}
	// Counters were reset on read: until the manager acknowledges a report,
	// these bytes exist only here. Persist so a SIGKILL / crash / power loss
	// does not silently drop them.
	s.persistPendingLocked()
	s.mu.Unlock()
}

// queryXrayStats reads and resets per-user counters through xray's
// StatsService (`xray api statsquery -reset`). Returns nothing when no xray
// process is managed.
func (s *Syncer) queryXrayStats() (map[uint]TrafficDelta, error) {
	if s.cfg.XrayBinary == "" {
		return nil, nil
	}
	s.mu.Lock()
	running := s.running
	port := s.apiPort
	s.mu.Unlock()
	if !running {
		return nil, nil
	}
	if port == 0 {
		port = defaultAPIPort
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, s.cfg.XrayBinary, "api", "statsquery",
		"-server=127.0.0.1:"+strconv.Itoa(port), "-pattern=user>>>", "-reset").Output()
	if err != nil {
		return nil, fmt.Errorf("xray api statsquery: %w", err)
	}
	return parseStatsQuery(out)
}

// parseStatsQuery parses `xray api statsquery` JSON output:
//
//	{"stat": [{"name": "user>>>u12>>>traffic>>>uplink", "value": "1024"}, ...]}
//
// value is an int64 that protojson may encode as a string or omit when zero.
// Client emails are "u{user_id}" (set by the manager's node config).
func parseStatsQuery(out []byte) (map[uint]TrafficDelta, error) {
	stats := make(map[uint]TrafficDelta)
	if len(bytes.TrimSpace(out)) == 0 {
		return stats, nil
	}
	var resp struct {
		Stat []struct {
			Name  string          `json:"name"`
			Value json.RawMessage `json:"value"`
		} `json:"stat"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("parse statsquery output: %w", err)
	}
	for _, st := range resp.Stat {
		parts := strings.Split(st.Name, ">>>")
		if len(parts) != 4 || parts[0] != "user" || parts[2] != "traffic" || !strings.HasPrefix(parts[1], "u") {
			continue
		}
		id, err := strconv.ParseUint(parts[1][1:], 10, 64)
		if err != nil || id == 0 {
			continue
		}
		raw := strings.Trim(strings.TrimSpace(string(st.Value)), `"`)
		var v int64
		if raw != "" && raw != "null" {
			v, err = strconv.ParseInt(raw, 10, 64)
			if err != nil {
				log.Printf("[sync] skipping stat %s: bad value %q", st.Name, raw)
				continue
			}
		}
		d := stats[uint(id)]
		switch parts[3] {
		case "uplink":
			d.Upload += v
		case "downlink":
			d.Download += v
		default:
			continue
		}
		stats[uint(id)] = d
	}
	return stats, nil
}

// configHash returns a content hash for change detection. Cryptographic on
// purpose: FNV would turn a collision into a permanently and silently skipped
// update; sha256 costs nothing here (once per changed config).
func configHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// pendingPath is where unacknowledged traffic deltas survive a restart.
func (s *Syncer) pendingPath() string {
	return filepath.Join(s.cfg.DataDir, "pending.json")
}

// pendingState is the pending.json on-disk format: unacknowledged deltas plus
// the in-flight report batch (its batch_id must survive a crash so a resend
// deduplicates instead of double-counting).
type pendingState struct {
	Pending     map[uint]TrafficDelta `json:"pending"`
	Outstanding *outstandingReport    `json:"outstanding,omitempty"`
}

// persistPendingLocked writes pending to disk (atomic temp+rename). Caller
// holds mu. Best-effort: a failed write only means a crash may drop traffic,
// never that accounting is corrupted.
func (s *Syncer) persistPendingLocked() {
	if s.cfg.DataDir == "" {
		return
	}
	data, err := json.Marshal(pendingState{Pending: s.pending, Outstanding: s.outstandingBatch})
	if err != nil {
		return
	}
	path := s.pendingPath()
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
	}
}

// loadPending restores pending deltas persisted by a previous run. Bytes that
// were read (reset) from xray but never acknowledged would otherwise vanish.
func (s *Syncer) loadPending() {
	data, err := os.ReadFile(s.pendingPath())
	if err != nil {
		return
	}
	var st pendingState
	// Pre-batch format was a bare map of deltas (no "pending" key).
	if err := json.Unmarshal(data, &st); err != nil || (st.Pending == nil && st.Outstanding == nil) {
		var legacy map[uint]TrafficDelta
		if lerr := json.Unmarshal(data, &legacy); lerr != nil {
			log.Printf("[sync] ignoring corrupt pending file %s: %v", s.pendingPath(), lerr)
			return
		}
		st = pendingState{Pending: legacy}
	}
	s.mu.Lock()
	for userID, d := range st.Pending {
		if userID == 0 || (d.Upload <= 0 && d.Download <= 0) {
			continue
		}
		p := s.pending[userID]
		p.Upload += d.Upload
		p.Download += d.Download
		s.pending[userID] = p
	}
	if st.Outstanding != nil && st.Outstanding.BatchID != "" && len(st.Outstanding.Entries) > 0 {
		s.outstandingBatch = st.Outstanding
	}
	n := len(s.pending)
	restoredBatch := s.outstandingBatch
	s.mu.Unlock()
	if n > 0 {
		log.Printf("[sync] restored unreported traffic for %d user(s) from %s", n, s.pendingPath())
	}
	if restoredBatch != nil {
		log.Printf("[sync] restored in-flight traffic batch %s (%d entries)", restoredBatch.BatchID, len(restoredBatch.Entries))
	}
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
	nodeID := s.nodeID
	if id, ok := meta["node_id"].(float64); ok && nodeID == 0 {
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
