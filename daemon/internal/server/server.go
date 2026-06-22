package server

import (
	"net/http"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ouyexiaogongzhu/airport/daemon/internal/config"
	"github.com/ouyexiaogongzhu/airport/daemon/internal/sync"
)

// Syncer defines the interface the server expects from the sync component.
type Syncer interface {
	GetLocalNodes() ([]sync.NodeConfig, error)
	LastSyncResult() (*sync.SyncResult, error)
}

// HealthResponse is returned by the health endpoint.
type HealthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}

// StatusResponse is returned by the /api/v1/status endpoint.
type StatusResponse struct {
	NodeID   uint             `json:"node_id"`
	LastSync *sync.SyncResult `json:"last_sync,omitempty"`
}

// TrafficResponse holds aggregated traffic data.
type TrafficResponse struct {
	TotalUp   int64 `json:"total_up"`
	TotalDown int64 `json:"total_down"`
	NodeCount int   `json:"node_count"`
}

// Server wraps the Fiber HTTP server and its dependencies.
type Server struct {
	app    *fiber.App
	cfg    *config.Config
	syncer Syncer
}

// New creates a new Server, registers routes, and returns it.
func New(cfg *config.Config, syncer Syncer) *Server {
	s := &Server{
		app:    fiber.New(fiber.Config{DisableStartupMessage: true}),
		cfg:    cfg,
		syncer: syncer,
	}
	s.registerRoutes()
	return s
}

// App returns the underlying Fiber app (useful for testing).
func (s *Server) App() *fiber.App {
	return s.app
}

// Start begins listening on the configured address.
func (s *Server) Start() error {
	return s.app.Listen(s.cfg.ListenAddr)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() error {
	return s.app.Shutdown()
}

func (s *Server) registerRoutes() {
	s.app.Get("/health", s.healthHandler)
	s.app.Get("/api/v1/status", s.statusHandler)
	s.app.Get("/api/v1/nodes", s.nodesHandler)
	s.app.Get("/api/v1/traffic", s.trafficHandler)
}

func (s *Server) healthHandler(c *fiber.Ctx) error {
	return c.JSON(HealthResponse{
		Status:    "ok",
		Service:   "daemon-api",
		Version:   "1.0.0",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) statusHandler(c *fiber.Ctx) error {
	lastSync, _ := s.syncer.LastSyncResult()
	return c.JSON(StatusResponse{
		NodeID:   s.cfg.NodeID,
		LastSync: lastSync,
	})
}

func (s *Server) nodesHandler(c *fiber.Ctx) error {
	nodes, err := s.syncer.GetLocalNodes()
	if err != nil {
		if os.IsNotExist(err) {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{
				"error":   "not_found",
				"message": "no nodes data available, sync may not have run yet",
			})
		}
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error":   "read_error",
			"message": err.Error(),
		})
	}
	return c.JSON(nodes)
}

func (s *Server) trafficHandler(c *fiber.Ctx) error {
	nodes, err := s.syncer.GetLocalNodes()
	if err != nil {
		// If no data yet, return empty aggregate.
		return c.JSON(TrafficResponse{TotalUp: 0, TotalDown: 0, NodeCount: 0})
	}

	var totalUp, totalDown int64
	for _, n := range nodes {
		totalUp += n.TrafficUp
		totalDown += n.TrafficDown
	}

	return c.JSON(TrafficResponse{
		TotalUp:   totalUp,
		TotalDown: totalDown,
		NodeCount: len(nodes),
	})
}
