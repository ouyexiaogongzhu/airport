package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/handler"
	"github.com/ouyexiaogongzhu/airport/manager/internal/middleware"
)

// TODO: Replace log with structured logging (zerolog or log/slog) for better observability.

func main() {
	// Initialize database
	dataDir := os.Getenv("DATA_DIR")
	db.Init(dataDir)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:   "RFPlay Airport Manager API",
		BodyLimit: 10 * 1024 * 1024, // 10MB max request body
	})

	// Global middleware
	app.Use(recover.New())
	// Deliberately no ${path}/${url} tag: node daemon tokens and client link
	// tokens live in the URL path (/node/:token/..., /client/links/:token) and
	// must not be written to logs in cleartext.
	app.Use(logger.New(logger.Config{
		Format: "${time} ${method} ${status} ${latency} ${ip}\n",
	}))

	// CORS whitelist
	corsOrigins := os.Getenv("CORS_ORIGINS")
	if corsOrigins == "" {
		corsOrigins = "http://localhost:5173,http://localhost:5174"
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins:     corsOrigins,
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-CSRF-Token,X-Client",
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
	v1.Get("/captcha", handler.CaptchaEndpoint)

	// Public routes (rate limited: 10 req/s per IP)
	public := v1.Group("/public", middleware.RateLimit(middleware.RateGroupPublic))
	public.Post("/register", handler.Register)
	public.Post("/login", handler.Login)
	public.Post("/token-login", handler.TokenLogin)
	public.Post("/payment/callback", handler.MockPayCallback)
	public.Post("/payment/callback/:provider", handler.PaymentCallback)

	// Client routes (public, rate limited: 10 req/s per IP)
	v1.Get("/client/config", middleware.RateLimit(middleware.RateGroupPublic), handler.GetClientConfig)
	v1.Get("/client/links/:token", middleware.RateLimit(middleware.RateGroupPublic), handler.GetLinksV2ray)
	v1.Get("/client/links/:token/clash", middleware.RateLimit(middleware.RateGroupPublic), handler.GetLinksClash)
	v1.Get("/client/links/:token/singbox", middleware.RateLimit(middleware.RateGroupPublic), handler.GetLinksSingbox)
	v1.Get("/client/links/:token/qrcode", middleware.RateLimit(middleware.RateGroupPublic), handler.GetLinksQRCode)

	// Client routes (JWT required, rate limited: 30 req/s per IP)
	client := v1.Group("/client", middleware.JWTProtected(), middleware.RateLimit(middleware.RateGroupAPI))
	client.Get("/subscription", handler.GetSubscription)

	// Web session auth (cookie-based; CSRF via double-submit header)
	auth := v1.Group("/auth", middleware.RateLimit(middleware.RateGroupPublic))
	auth.Get("/csrf", handler.GetCSRFToken)
	auth.Get("/validate", middleware.WebAuth("session"), handler.ValidateSession)
	auth.Post("/refresh", middleware.WebAuth("session"), handler.Refresh)
	auth.Post("/logout", middleware.WebAuth("session"), handler.Logout)

	// Admin session auth (cookie-based)
	adminAuth := v1.Group("/admin/auth", middleware.RateLimit(middleware.RateGroupPublic))
	adminAuth.Post("/login", handler.AdminLogin)
	adminAuth.Post("/logout", handler.AdminLogout)
	adminAuth.Get("/csrf", handler.GetCSRFToken)

	// Node daemon endpoints (authenticated by per-node token + HMAC signature)
	nodeRoutes := v1.Group("/node")
	nodeRoutes.Use(middleware.NodeHMAC())
	nodeRoutes.Get("/:token/config", handler.GetNodeConfigForDaemon)
	nodeRoutes.Post("/:token/traffic/report", handler.ReportNodeTraffic)

	// Web routes (cookie session required, rate limited: 30 req/s per IP)
	web := v1.Group("/web", middleware.WebAuth("session"), middleware.RateLimit(middleware.RateGroupAPI))
	web.Get("/client-token", handler.GetClientToken)
	web.Post("/client-token/regenerate", middleware.WebCSRF("csrf"), handler.RegenerateClientToken)

	// User routes (cookie session required, rate limited: 30 req/s per IP)
	user := v1.Group("/user", middleware.WebAuth("session"), middleware.RateLimit(middleware.RateGroupAPI))
	user.Get("/profile", handler.GetProfile)
	user.Put("/profile", middleware.WebCSRF("csrf"), handler.UpdateProfile)
	user.Post("/orders", middleware.WebCSRF("csrf"), handler.CreateOrder)
	user.Get("/orders", handler.ListOrders)
	user.Get("/orders/:id", handler.GetOrder)

	// Admin routes (admin session cookie + AdminOnly, rate limited: 30 req/s per IP)
	admin := v1.Group("/admin", middleware.WebAuth("admin_session"), middleware.AdminOnly(), middleware.RateLimit(middleware.RateGroupAPI))
	admin.Get("/users", handler.ListUsers)
	admin.Get("/users/:id", handler.GetUser)
	admin.Put("/users/:id", middleware.WebCSRF("admin_csrf"), handler.UpdateUser)
	admin.Get("/orders", handler.AdminListOrders)
	admin.Get("/orders/:id", handler.AdminGetOrder)
	admin.Post("/orders/:id/refund", middleware.WebCSRF("admin_csrf"), handler.AdminRefundOrder)
	admin.Get("/stats", handler.GetAdminStats)

	// Node management (admin)
	nodes := admin.Group("/nodes")
	nodes.Post("/", middleware.WebCSRF("admin_csrf"), handler.CreateNode)
	nodes.Get("/", handler.ListNode)
	nodes.Get("/:id", handler.GetNode)
	nodes.Put("/:id", middleware.WebCSRF("admin_csrf"), handler.UpdateNode)
	nodes.Delete("/:id", middleware.WebCSRF("admin_csrf"), handler.DeleteNode)
	nodes.Get("/:id/config", handler.GenerateNodeConfig)
	nodes.Post("/:id/token", middleware.WebCSRF("admin_csrf"), handler.GenerateNodeToken)

	// Traffic (admin)
	traffic := admin.Group("/traffic")
	traffic.Post("/report", middleware.WebCSRF("admin_csrf"), handler.ReportTraffic)
	traffic.Get("/stats", handler.GetTrafficStats)

	// Product management (admin)
	products := admin.Group("/products")
	products.Post("/", middleware.WebCSRF("admin_csrf"), handler.CreateProduct)
	products.Get("/", handler.ListProducts)
	products.Put("/:id", middleware.WebCSRF("admin_csrf"), handler.UpdateProduct)
	products.Delete("/:id", middleware.WebCSRF("admin_csrf"), handler.DeleteProduct)

	// Public product list (active only)
	v1.Get("/products", handler.ListActiveProducts)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Manager API starting on :%s", port)

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down gracefully...")
		if err := app.Shutdown(); err != nil {
			log.Printf("error during shutdown: %v", err)
		}
	}()

	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
