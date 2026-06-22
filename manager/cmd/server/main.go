package main

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/handler"
	"github.com/ouyexiaogongzhu/airport/manager/internal/middleware"
)

func main() {
	// Initialize database
	dataDir := os.Getenv("DATA_DIR")
	db.Init(dataDir)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName: "RFPlay Airport Manager API",
	})

	// Global middleware
	app.Use(recover.New())
	app.Use(logger.New())

	// CORS whitelist
	corsOrigins := os.Getenv("CORS_ORIGINS")
	if corsOrigins == "" {
		corsOrigins = "http://localhost:5173,http://localhost:5174"
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins:     corsOrigins,
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization",
		AllowCredentials: true,
	}))

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "manager-api",
		})
	})

	// API v1 routes
	v1 := app.Group("/api/v1")

	// Public routes (rate limited: 10 req/s per IP)
	public := v1.Group("/public", middleware.RateLimit(middleware.RateGroupPublic))
	public.Post("/register", handler.Register)
	public.Post("/login", handler.Login)
	public.Post("/token-login", handler.TokenLogin)
	public.Post("/payment/callback", handler.MockPayCallback)

	// Client routes (public, rate limited: 10 req/s per IP)
	v1.Get("/client/config", middleware.RateLimit(middleware.RateGroupPublic), handler.GetClientConfig)
	v1.Get("/client/links/:token", middleware.RateLimit(middleware.RateGroupPublic), handler.GetLinksV2ray)
	v1.Get("/client/links/:token/clash", middleware.RateLimit(middleware.RateGroupPublic), handler.GetLinksClash)
	v1.Get("/client/links/:token/singbox", middleware.RateLimit(middleware.RateGroupPublic), handler.GetLinksSingbox)
	v1.Get("/client/links/:token/qrcode", middleware.RateLimit(middleware.RateGroupPublic), handler.GetLinksQRCode)

	// Client routes (JWT required, rate limited: 30 req/s per IP)
	client := v1.Group("/client", middleware.JWTProtected(), middleware.RateLimit(middleware.RateGroupAPI))
	client.Get("/subscription", handler.GetSubscription)

	// Web routes (JWT required, rate limited: 30 req/s per IP)
	web := v1.Group("/web", middleware.JWTProtected(), middleware.RateLimit(middleware.RateGroupAPI))
	web.Get("/client-token", handler.GetClientToken)
	web.Post("/client-token/regenerate", handler.RegenerateClientToken)

	// User routes (JWT required, rate limited: 30 req/s per IP)
	user := v1.Group("/user", middleware.JWTProtected(), middleware.RateLimit(middleware.RateGroupAPI))
	user.Get("/profile", handler.GetProfile)
	user.Put("/profile", handler.UpdateProfile)
	user.Post("/orders", handler.CreateOrder)
	user.Get("/orders", handler.ListOrders)

	// Admin routes (JWT + AdminOnly, rate limited: 30 req/s per IP)
	admin := v1.Group("/admin", middleware.JWTProtected(), middleware.AdminOnly(), middleware.RateLimit(middleware.RateGroupAPI))
	admin.Get("/users", handler.ListUsers)
	admin.Get("/users/:id", handler.GetUser)

	// Node management (admin)
	nodes := admin.Group("/nodes")
	nodes.Post("/", handler.CreateNode)
	nodes.Get("/", handler.ListNode)
	nodes.Get("/:id", handler.GetNode)
	nodes.Put("/:id", handler.UpdateNode)
	nodes.Delete("/:id", handler.DeleteNode)

	// Traffic (admin)
	traffic := admin.Group("/traffic")
	traffic.Post("/report", handler.ReportTraffic)
	traffic.Get("/stats", handler.GetTrafficStats)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Manager API starting on :%s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
