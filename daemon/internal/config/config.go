package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Config holds the daemon configuration.
type Config struct {
	// NodeID is this daemon's unique identifier (matches Manager node ID).
	NodeID uint `json:"node_id"`

	// ManagerURL is the base URL of the Manager API (e.g. http://localhost:8080).
	ManagerURL string `json:"manager_url"`

	// ManagerToken is the JWT token used to authenticate with the Manager API.
	ManagerToken string `json:"manager_token"`

	// SyncInterval is how often to sync node config from the Manager.
	SyncInterval time.Duration `json:"sync_interval"`

	// ListenAddr is the address the daemon HTTP API listens on.
	ListenAddr string `json:"listen_addr"`

	// DataDir is the directory for storing local config and data.
	DataDir string `json:"data_dir"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		NodeID:        1,
		ManagerURL:    "http://localhost:8080",
		ManagerToken:  "",
		SyncInterval:  60 * time.Second,
		ListenAddr:    ":9090",
		DataDir:       "./data",
	}
}

// LoadConfig reads configuration from a JSON file.
// Returns a default config merged with file contents.
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	return cfg, nil
}

// SaveConfig writes the config to a JSON file.
func (c *Config) SaveConfig(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}

// Validate checks the config for required fields.
func (c *Config) Validate() error {
	if c.NodeID == 0 {
		return fmt.Errorf("node_id is required")
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
