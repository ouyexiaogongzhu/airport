package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Default placeholder values. Validate rejects them so a gateway started
// without a real config file fails fast instead of polling a bogus manager.
const (
	DefaultManagerURL   = "http://localhost:8080"
	DefaultManagerToken = "default-token"
)

// Config holds the gateway configuration.
type Config struct {
	// NodeID is informational only: the authoritative node id comes from the
	// manager's config response (the token identifies the node).
	NodeID       uint          `json:"node_id,omitempty"`
	ManagerURL   string        `json:"manager_url"`
	ManagerToken string        `json:"manager_token"`
	SyncInterval time.Duration `json:"sync_interval"`
	DataDir      string        `json:"data_dir"`
	ListenAddr   string        `json:"listen_addr"`
	LogLevel     string        `json:"log_level"`
	XrayBinary   string        `json:"xray_binary,omitempty"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		ManagerURL:   DefaultManagerURL,
		ManagerToken: DefaultManagerToken,
		SyncInterval: 30 * time.Second,
		DataDir:      "/var/lib/airport",
		ListenAddr:   "127.0.0.1:9090",
		LogLevel:     "info",
	}
}

// LoadConfig reads a JSON config file at path. If path does not exist,
// it returns DefaultConfig with no error. Invalid JSON returns an error.
func LoadConfig(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// SaveConfig writes cfg as pretty-printed JSON to path.
func SaveConfig(cfg *Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// Validate checks that required fields are set. Returns an error if invalid.
func (c *Config) Validate() error {
	if c.ManagerURL == "" {
		return fmt.Errorf("manager_url is required")
	}
	if c.ManagerURL == DefaultManagerURL {
		return fmt.Errorf("manager_url is still the default %q; set the real manager URL", DefaultManagerURL)
	}
	if c.ManagerToken == "" {
		return fmt.Errorf("manager_token is required")
	}
	if c.ManagerToken == DefaultManagerToken {
		return fmt.Errorf("manager_token is still the default %q; set the node token from the admin panel", DefaultManagerToken)
	}
	if c.SyncInterval <= 0 {
		return fmt.Errorf("sync_interval must be positive")
	}
	// time.Duration unmarshals JSON numbers as NANOSECONDS: a bare 60 becomes
	// 60ns, passes naive validation and hammers the manager in a tight loop.
	if c.SyncInterval < time.Second {
		return fmt.Errorf("sync_interval must be at least 1s (got %s); note the value is in nanoseconds, e.g. 60000000000 = 60s", c.SyncInterval)
	}
	// The local HTTP API has no authentication, so it must never be exposed.
	host, _, err := net.SplitHostPort(c.ListenAddr)
	if err != nil {
		return fmt.Errorf("invalid listen_addr %q: %w", c.ListenAddr, err)
	}
	if host != "localhost" {
		if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("listen_addr must bind a loopback address (e.g. 127.0.0.1:9090), got %q", c.ListenAddr)
		}
	}
	return nil
}
