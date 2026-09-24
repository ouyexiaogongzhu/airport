package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ouyexiaogongzhu/airport/daemon/internal/config"
	"github.com/ouyexiaogongzhu/airport/daemon/internal/server"
	"github.com/ouyexiaogongzhu/airport/daemon/internal/sync"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[daemon] RFPlay Node Daemon starting...")

	// Load configuration from file or environment
	configPath := "daemon.json"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}
	if envPath := os.Getenv("DAEMON_CONFIG"); envPath != "" {
		configPath = envPath
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("[daemon] failed to load config: %v", err)
	}

	// Override from environment variables (takes precedence)
	if v := os.Getenv("DAEMON_MANAGER_URL"); v != "" {
		cfg.ManagerURL = v
	}
	if v := os.Getenv("DAEMON_MANAGER_TOKEN"); v != "" {
		cfg.ManagerToken = v
	}
	if v := os.Getenv("DAEMON_NODE_ID"); v != "" {
		var id uint
		if _, err := fmt.Sscanf(v, "%d", &id); err == nil {
			cfg.NodeID = id
		}
	}
	if v := os.Getenv("DAEMON_LISTEN_ADDR"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("DAEMON_XRAY_BINARY"); v != "" {
		cfg.XrayBinary = v
	}
	if v := os.Getenv("DAEMON_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("[daemon] invalid configuration: %v", err)
	}

	log.Printf("[daemon] manager=%s listen=%s sync=%s (node_id comes from the manager)",
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

	log.Println("[daemon] all services started")

	// Wait for shutdown signal (or a server failure). Cleanup runs explicitly
	// so xray is never left behind as an orphan.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	exitCode := 0
	select {
	case <-sigCh:
		log.Println("[daemon] shutting down...")
	case err := <-srvErr:
		log.Printf("[daemon] server error: %v", err)
		exitCode = 1
	}

	_ = srv.Shutdown()
	syncer.Stop()
	os.Exit(exitCode)
}
