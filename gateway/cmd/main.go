package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ouyexiaogongzhu/airport/gateway/internal/config"
	"github.com/ouyexiaogongzhu/airport/gateway/internal/server"
	"github.com/ouyexiaogongzhu/airport/gateway/internal/sync"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[gateway] RFPlay Node Gateway starting...")

	// Load configuration from file or environment
	configPath := "gateway.json"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}
	if envPath := firstEnv("GATEWAY_CONFIG", "DAEMON_CONFIG"); envPath != "" {
		configPath = envPath
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("[gateway] failed to load config: %v", err)
	}

	// Override from environment variables (takes precedence).
	// GATEWAY_* is preferred; DAEMON_* is accepted for upgrade compatibility.
	if v := firstEnv("GATEWAY_MANAGER_URL", "DAEMON_MANAGER_URL"); v != "" {
		cfg.ManagerURL = v
	}
	if v := firstEnv("GATEWAY_MANAGER_TOKEN", "DAEMON_MANAGER_TOKEN"); v != "" {
		cfg.ManagerToken = v
	}
	if v := firstEnv("GATEWAY_NODE_ID", "DAEMON_NODE_ID"); v != "" {
		var id uint
		if _, err := fmt.Sscanf(v, "%d", &id); err == nil {
			cfg.NodeID = id
		}
	}
	if v := firstEnv("GATEWAY_LISTEN_ADDR", "DAEMON_LISTEN_ADDR"); v != "" {
		cfg.ListenAddr = v
	}
	if v := firstEnv("GATEWAY_XRAY_BINARY", "DAEMON_XRAY_BINARY"); v != "" {
		cfg.XrayBinary = v
	}
	if v := firstEnv("GATEWAY_DATA_DIR", "DAEMON_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("[gateway] invalid configuration: %v", err)
	}

	log.Printf("[gateway] manager=%s listen=%s sync=%s (node_id comes from the manager)",
		cfg.ManagerURL, cfg.ListenAddr, cfg.SyncInterval)

	// Create syncer; Stop also terminates the managed xray process.
	syncer := sync.NewSyncer(cfg)
	go syncer.Start()

	// Create HTTP server
	srv := server.New(cfg, syncer)
	srvErr := make(chan error, 1)
	go func() {
		srvErr <- srv.Start()
	}()

	log.Println("[gateway] all services started")

	// Wait for shutdown signal (or a server failure). Cleanup runs explicitly
	// so xray is never left behind as an orphan.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	exitCode := 0
	select {
	case <-sigCh:
		log.Println("[gateway] shutting down...")
	case err := <-srvErr:
		log.Printf("[gateway] server error: %v", err)
		exitCode = 1
	}

	_ = srv.Shutdown()
	syncer.Stop()
	os.Exit(exitCode)
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}
