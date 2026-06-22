package server

import (
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/ouyexiaogongzhu/airport/daemon/internal/config"
	"github.com/ouyexiaogongzhu/airport/daemon/internal/sync"
)

// Server is the daemon's HTTP API server, providing endpoints for Flutter apps.
type Server struct {
	cfg    *config.Config
	syncer *sync.Syncer
	app    *fiber.App
}

// New creates a new Server instance.
func New(cfg *config.Config, syncer *sync.Syncer) *Server {
	return &Server{
		cfg:    cfg,
		syncer: syncer,
	}
}

// Start initialises routes and begins listening.
func (s *Server) Start() error {
	s.app = fiber.New(fiber.Config{
		AppName:      "RFPlay Airport Daemon API",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	})

	// Global middleware
	s.app.Use(recover.New())
	s.app.Use(logger.New())
	s.app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	// Health check
	s.app.Get("/health", s.handleHealth)

	// API routes (v1)
	v1 := s.app.Group("/api/v1")

	// Status endpoint
	v1.Get("/status", s.handleStatus)

	// Node info
	v1.Get("/nodes", s.handleNodes)

	// Traffic summary
	v1.Get("/traffic", s.handleTraffic)

	log.Printf("[server] listening on %s", s.cfg.ListenAddr)
	return s.app.Listen(s.cfg.ListenAddr)
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown() error {
	if s.app != nil {
		return s.app.Shutdown()
	}
	return nil
}

// handleHealth returns a simple health check response.
func (s *Server) handleHealth(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":  "ok",
		"service": "daemon-api",
		"node_id": s.cfg.NodeID,
		"time":    time.Now().Format(time.RFC3339),
	})
}

// handleStatus returns the daemon's current status.
func (s *Server) handleStatus(c *fiber.Ctx) error {
	syncResult, err := s.syncer.LastSyncResult()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to get sync status",
		})
	}

	return c.JSON(fiber.Map{
		"node_id":       s.cfg.NodeID,
		"manager_url":   s.cfg.ManagerURL,
		"sync_interval": s.cfg.SyncInterval.String(),
		"sync":          syncResult,
		"uptime":        time.Now().Format(time.RFC3339),
	})
}

// handleNodes returns the locally cached node configuration.
func (s *Server) handleNodes(c *fiber.Ctx) error {
	nodes, err := s.syncer.GetLocalNodes()
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   "no node config available",
			"message": "sync has not completed yet",
		})
	}

	return c.JSON(fiber.Map{
		"data":  nodes,
		"count": len(nodes),
	})
}

// handleTraffic returns traffic summary from the local cache.
func (s *Server) handleTraffic(c *fiber.Ctx) error {
	nodes, err := s.syncer.GetLocalNodes()
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   "no traffic data available",
			"message": "sync has not completed yet",
		})
	}

	// Aggregate traffic across all nodes
	totalUp := int64(0)
	totalDown := int64(0)
	for _, node := range nodes {
		totalUp += node.TrafficUp
		totalDown += node.TrafficDown
	}

	// Per-node breakdown
	type nodeTraffic struct {
		NodeID      uint   `json:"node_id"`
		Name        string `json:"name"`
		TrafficUp   int64  `json:"traffic_up"`
		TrafficDown int64  `json:"traffic_down"`
	}

	breakdown := make([]nodeTraffic, 0, len(nodes))
	for _, node := range nodes {
		breakdown = append(breakdown, nodeTraffic{
			NodeID:      node.ID,
			Name:        node.Name,
			TrafficUp:   node.TrafficUp,
			TrafficDown: node.TrafficDown,
		})
	}

	return c.JSON(fiber.Map{
		"total_up":     fmt.Sprintf("%d", totalUp),
		"total_down":   fmt.Sprintf("%d", totalDown),
		"total_human":  fmt.Sprintf("%.2f GB", float64(totalUp+totalDown)/1073741824),
		"breakdown":    breakdown,
	})
}
