package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// validConfig returns a config that passes Validate.
func validConfig() *Config {
	cfg := DefaultConfig()
	cfg.ManagerURL = "https://api.example.com"
	cfg.ManagerToken = "nd_test"
	return cfg
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.NodeID != 0 {
		t.Errorf("expected NodeID=0 (taken from manager), got %d", cfg.NodeID)
	}
	if cfg.ManagerURL != "http://localhost:8080" {
		t.Errorf("expected ManagerURL=http://localhost:8080, got %s", cfg.ManagerURL)
	}
	if cfg.ManagerToken != "default-token" {
		t.Errorf("expected ManagerToken=default-token, got %s", cfg.ManagerToken)
	}
	if cfg.SyncInterval != 30*time.Second {
		t.Errorf("expected SyncInterval=30s, got %v", cfg.SyncInterval)
	}
	if cfg.DataDir != "/var/lib/airport" {
		t.Errorf("expected DataDir=/var/lib/airport, got %s", cfg.DataDir)
	}
	if cfg.ListenAddr != "127.0.0.1:9090" {
		t.Errorf("expected ListenAddr=127.0.0.1:9090, got %s", cfg.ListenAddr)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected LogLevel=info, got %s", cfg.LogLevel)
	}
}

func TestLoadConfig_FileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	original := DefaultConfig()
	original.NodeID = 42
	original.ManagerURL = "http://manager.example.com"
	original.ManagerToken = "secret123"

	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg.NodeID != 42 {
		t.Errorf("expected NodeID=42, got %d", cfg.NodeID)
	}
	if cfg.ManagerURL != "http://manager.example.com" {
		t.Errorf("expected ManagerURL=http://manager.example.com, got %s", cfg.ManagerURL)
	}
	if cfg.ManagerToken != "secret123" {
		t.Errorf("expected ManagerToken=secret123, got %s", cfg.ManagerToken)
	}
}

func TestLoadConfig_FileNotExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig for missing file returned error: %v", err)
	}
	if cfg.ManagerToken != DefaultManagerToken {
		t.Errorf("expected default ManagerToken, got %s", cfg.ManagerToken)
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{invalid json}"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestSaveConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "saved.json")

	cfg := DefaultConfig()
	cfg.NodeID = 99
	cfg.ManagerURL = "https://saved.example.com"

	if err := SaveConfig(cfg, path); err != nil {
		t.Fatalf("SaveConfig error: %v", err)
	}

	// Verify file exists and is valid JSON
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("saved config is not valid JSON: %v", err)
	}
	if loaded.NodeID != 99 {
		t.Errorf("expected NodeID=99, got %d", loaded.NodeID)
	}
	if loaded.ManagerURL != "https://saved.example.com" {
		t.Errorf("expected ManagerURL=https://saved.example.com, got %s", loaded.ManagerURL)
	}
}

func TestValidate_Valid(t *testing.T) {
	cfg := validConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid config should pass: %v", err)
	}
	// node_id is optional: it comes from the manager's config response.
	cfg.NodeID = 0
	if err := cfg.Validate(); err != nil {
		t.Fatalf("node_id=0 should pass: %v", err)
	}
	for _, addr := range []string{"localhost:9090", "[::1]:9090", "127.0.0.2:9090"} {
		cfg.ListenAddr = addr
		if err := cfg.Validate(); err != nil {
			t.Errorf("loopback listen_addr %q should pass: %v", addr, err)
		}
	}
}

func TestValidate_RejectsDefaults(t *testing.T) {
	if err := DefaultConfig().Validate(); err == nil {
		t.Fatal("expected error for the default config")
	}
	cfg := validConfig()
	cfg.ManagerToken = DefaultManagerToken
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for default manager_token")
	}
	cfg = validConfig()
	cfg.ManagerURL = DefaultManagerURL
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for default manager_url")
	}
}

func TestValidate_RejectsPublicListenAddr(t *testing.T) {
	for _, addr := range []string{":9090", "0.0.0.0:9090", "[::]:9090", "203.0.113.5:9090", "9090"} {
		cfg := validConfig()
		cfg.ListenAddr = addr
		if err := cfg.Validate(); err == nil {
			t.Errorf("expected error for listen_addr %q", addr)
		}
	}
}

func TestValidate_MissingManagerURL(t *testing.T) {
	cfg := validConfig()
	cfg.ManagerURL = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty ManagerURL")
	}
}

func TestValidate_MissingManagerToken(t *testing.T) {
	cfg := validConfig()
	cfg.ManagerToken = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty ManagerToken")
	}
}

func TestValidate_NegativeSyncInterval(t *testing.T) {
	cfg := validConfig()
	cfg.SyncInterval = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for SyncInterval=0")
	}

	cfg.SyncInterval = -1
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for SyncInterval=-1")
	}
}
