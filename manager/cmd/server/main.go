package main

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
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

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "manager-api",
		})
	})

	// API v1 routes
	v1 := app.Group("/api/v1")

	// Public routes (no auth required)
	public := v1.Group("/public")
	public.Post("/register", handler.Register)
	public.Post("/login", handler.Login)

	// User routes (JWT required)
	user := v1.Group("/user", middleware.JWTProtected())
	user.Get("/profile", handler.GetProfile)
	user.Put("/profile", handler.UpdateProfile)

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
