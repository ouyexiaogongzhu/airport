//go:build seed

package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("failed to create data dir: %v", err)
	}

	dbPath := filepath.Join(dataDir, "manager.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// Auto migrate so tables exist
	if err := db.AutoMigrate(
		&model.User{},
		&model.Product{},
	); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}

	// Note: User accounts are NOT seeded here.
	// Users must be created through the /api/v1/public/register endpoint.
	// Run the server and use the API to create users.

	// --- Seed products ---

	products := []model.Product{
		{Name: "Basic 1 month 10GB", Type: "monthly", Price: 5.99, Stock: 999, Status: "active"},
		{Name: "Standard 1 month 50GB", Type: "monthly", Price: 12.99, Stock: 999, Status: "active"},
		{Name: "Premium 1 month 200GB", Type: "monthly", Price: 29.99, Stock: 999, Status: "active"},
	}
	for _, p := range products {
		var existing model.Product
		if err := db.Where("name = ?", p.Name).First(&existing).Error; err == nil {
			log.Printf("product already exists, skipped: name=%s", p.Name)
			continue
		}
		if err := db.Create(&p).Error; err != nil {
			log.Fatalf("failed to create product %q: %v", p.Name, err)
		}
		log.Printf("created product: name=%s price=%.2f stock=%d", p.Name, p.Price, p.Stock)
	}

	log.Println("seed complete")
}
