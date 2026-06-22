package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Config holds the daemon configuration.
type Config struct {
	NodeID         uint          `json:"node_id"`
	ManagerURL     string        `json:"manager_url"`
	ManagerToken   string        `json:"manager_token"`
	SyncInterval   time.Duration `json:"sync_interval"`
	DataDir        string        `json:"data_dir"`
	ListenAddr     string        `json:"listen_addr"`
	LogLevel       string        `json:"log_level"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		NodeID:       1,
		ManagerURL:   "http://localhost:8080",
		ManagerToken: "default-token",
		SyncInterval: 30 * time.Second,
		DataDir:      "/var/lib/airport",
		ListenAddr:   ":9090",
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
	if c.NodeID <= 0 {
		return fmt.Errorf("node_id must be positive")
	}
	if c.ManagerURL == "" {
		return fmt.Errorf("manager_url is required")
	}
	if c.ManagerToken == "" {
		return fmt.Errorf("manager_token is required")
	}
	if c.SyncInterval <= 0 {
		return fmt.Errorf("sync_interval must be positive")
	}
	return nil
}
