package sync

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ouyexiaogongzhu/airport/daemon/internal/config"
)

// Syncer handles periodic synchronisation of node configuration from the Manager API.
type Syncer struct {
	cfg    *config.Config
	client *http.Client
	stopCh chan struct{}
}

// NodeConfig is the node configuration fetched from the Manager.
// It mirrors the Manager's Node model for local storage.
type NodeConfig struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Address       string `json:"address"`
	Port          int    `json:"port"`
	Protocol      string `json:"protocol"`
	Status        string `json:"status"`
	TrafficUp     int64  `json:"traffic_up"`
	TrafficDown   int64  `json:"traffic_down"`
	UserID        uint   `json:"user_id"`
}

// SyncResult holds the result of a sync operation.
type SyncResult struct {
	Success     bool   `json:"success"`
	NodeCount   int    `json:"node_count"`
	Message     string `json:"message"`
	SyncedAt    string `json:"synced_at"`
}

// NewSyncer creates a new Syncer.
func NewSyncer(cfg *config.Config) *Syncer {
	return &Syncer{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		stopCh: make(chan struct{}),
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

// Sync performs a single sync: fetches node config from Manager and writes to disk.
func (s *Syncer) Sync() error {
	nodes, err := s.fetchNodes()
	if err != nil {
		return fmt.Errorf("fetch nodes: %w", err)
	}

	if err := s.writeLocalConfig(nodes); err != nil {
		return fmt.Errorf("write local config: %w", err)
	}

	log.Printf("[sync] synced %d nodes from manager", len(nodes))
	return nil
}

// fetchNodes calls the Manager API to get the list of nodes.
func (s *Syncer) fetchNodes() ([]NodeConfig, error) {
	url := fmt.Sprintf("%s/api/v1/admin/nodes", s.cfg.ManagerURL)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Authenticate with JWT token
	req.Header.Set("Authorization", "Bearer "+s.cfg.ManagerToken)
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

	var nodes []NodeConfig
	if err := json.Unmarshal(body, &nodes); err != nil {
		return nil, fmt.Errorf("parse response: %w (body: %s)", err, string(body))
	}

	return nodes, nil
}

// writeLocalConfig saves the fetched nodes to a local JSON file.
func (s *Syncer) writeLocalConfig(nodes []NodeConfig) error {
	// Ensure data directory exists
	if err := os.MkdirAll(s.cfg.DataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	// Write nodes config
	path := filepath.Join(s.cfg.DataDir, "nodes.json")
	data, err := json.MarshalIndent(nodes, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal nodes: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write nodes: %w", err)
	}

	return nil
}

// LastSyncResult returns the last sync result from the on-disk state.
func (s *Syncer) LastSyncResult() (*SyncResult, error) {
	nodes, err := s.readLocalConfig()
	if err != nil {
		return &SyncResult{
			Success:  false,
			Message:  fmt.Sprintf("no local config: %v", err),
			SyncedAt: time.Now().Format(time.RFC3339),
		}, nil
	}

	return &SyncResult{
		Success:   true,
		NodeCount: len(nodes),
		Message:   "ok",
		SyncedAt:  time.Now().Format(time.RFC3339),
	}, nil
}

// readLocalConfig reads the locally stored nodes config.
func (s *Syncer) readLocalConfig() ([]NodeConfig, error) {
	path := filepath.Join(s.cfg.DataDir, "nodes.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var nodes []NodeConfig
	if err := json.Unmarshal(data, &nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

// GetLocalNodes returns the locally cached node configurations.
func (s *Syncer) GetLocalNodes() ([]NodeConfig, error) {
	return s.readLocalConfig()
}
