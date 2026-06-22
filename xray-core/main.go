package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/ouyexiaogongzhu/airport/xray-core/ratelimit"
	"github.com/ouyexiaogongzhu/airport/xray-core/verify"
)

// XrayConfig is the minimal configuration structure for our fork.
// In production, this would be the full Xray-core JSON config.
type XrayConfig struct {
	Log        *LogConfig           `json:"log,omitempty"`
	Inbounds   []InboundConfig      `json:"inbounds"`
	Outbounds  []OutboundConfig     `json:"outbounds"`
	VerifyURL string                `json:"verify_url"`  // Manager API base for token verification
	RateLimit  *RateLimitConfig     `json:"rate_limit,omitempty"`
}

type LogConfig struct {
	Access   string `json:"access,omitempty"`
	Error    string `json:"error,omitempty"`
	Loglevel string `json:"loglevel,omitempty"`
}

type InboundConfig struct {
	Port        int                `json:"port"`
	Protocol    string             `json:"protocol"`
	StreamSettings *StreamSettings `json:"streamSettings,omitempty"`
	Settings    json.RawMessage    `json:"settings,omitempty"`
	Tag         string             `json:"tag,omitempty"`
}

type StreamSettings struct {
	Network       string         `json:"network,omitempty"`
	WSSettings    *WSSettings    `json:"wsSettings,omitempty"`
	TLSSettings   *TLSSettings   `json:"tlsSettings,omitempty"`
}

type WSSettings struct {
	Path string `json:"path,omitempty"`
	Host string `json:"host,omitempty"`
}

type TLSSettings struct {
	Certs []CertConfig `json:"certificates,omitempty"`
}

type CertConfig struct {
	CertFile string `json:"certificateFile,omitempty"`
	KeyFile  string `json:"keyFile,omitempty"`
}

type OutboundConfig struct {
	Protocol string          `json:"protocol"`
	Tag      string          `json:"tag,omitempty"`
}

type RateLimitConfig struct {
	Enabled bool `json:"enabled"`
	// Default rate: N requests per second per user
	Rate  float64 `json:"rate"`
	Burst int     `json:"burst"`
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[Xray-core fork] starting...")

	// Load configuration
	configPath := "config.json"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("failed to read config %s: %v", configPath, err)
	}

	var cfg XrayConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("failed to parse config: %v", err)
	}

	// Initialise rate limiter if configured
	var limiter *ratelimit.UserRateLimiter
	if cfg.RateLimit != nil && cfg.RateLimit.Enabled {
		limiter = ratelimit.NewUserRateLimiter(cfg.RateLimit.Rate, cfg.RateLimit.Burst)
		log.Printf("[rate-limit] enabled: rate=%.1f/s burst=%d", cfg.RateLimit.Rate, cfg.RateLimit.Burst)
	}

	// Start verify callback server (inbound connection hook)
	verifyServer := &verify.Server{
		ManagerURL: cfg.VerifyURL,
		Limiter:    limiter,
	}

	// Register the verify handler on the Xray inbound connection path.
	// In a real Xray-core fork, this would hook into the inbound handler's
	// connection-accept chain via an Xray plugin or custom protocol handler.
	// For this demo, we expose an HTTP endpoint that the inbound handler calls.
	http.HandleFunc("/verify", verifyServer.HandleVerify)

	// Start verify listener on an internal port (not exposed to clients)
	verifyPort := "1099"
	go func() {
		log.Printf("[verify] listening on :%s", verifyPort)
		if err := http.ListenAndServe(":"+verifyPort, nil); err != nil {
			log.Fatalf("verify server error: %v", err)
		}
	}()

	// Print configured inbounds
	fmt.Println("=== Xray-core fork configuration ===")
	fmt.Printf("Verify URL: %s\n", cfg.VerifyURL)
	for _, in := range cfg.Inbounds {
		network := "tcp"
		if in.StreamSettings != nil && in.StreamSettings.Network != "" {
			network = in.StreamSettings.Network
		}
		fmt.Printf("Inbound :%d/%s (%s) tag=%s\n", in.Port, in.Protocol, network, in.Tag)
	}
	fmt.Println("====================================")

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("[Xray-core fork] shutting down...")
}
