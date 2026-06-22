package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
	"golang.org/x/crypto/bcrypt"
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

	// --- Seed users ---

	adminHash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("failed to hash admin password: %v", err)
	}
	admin := model.User{
		Username:     "admin",
		PasswordHash: string(adminHash),
		Role:         "admin",
		Status:       "active",
		Balance:      0,
	}
	res := db.Where("username = ?", admin.Username).FirstOrCreate(&admin)
	if res.Error != nil {
		log.Fatalf("failed to create admin user: %v", res.Error)
	}
	if res.RowsAffected > 0 {
		// Set a placeholder client_token for the admin
		if err := db.Model(&admin).Update("client_token", "rf_seed_admin_token_placeholder").Error; err != nil {
			log.Printf("warning: failed to set admin client_token: %v", err)
		}
		log.Printf("created admin user: username=%s role=%s", admin.Username, admin.Role)
	} else {
		log.Printf("admin user already exists, skipped")
	}

	testHash, err := bcrypt.GenerateFromPassword([]byte("test123"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("failed to hash testuser password: %v", err)
	}
	testUser := model.User{
		Username:     "testuser",
		PasswordHash: string(testHash),
		Role:         "user",
		Status:       "active",
		Balance:      0,
	}
	res = db.Where("username = ?", testUser.Username).FirstOrCreate(&testUser)
	if res.Error != nil {
		log.Fatalf("failed to create test user: %v", res.Error)
	}
	if res.RowsAffected > 0 {
		log.Printf("created user: username=%s role=%s", testUser.Username, testUser.Role)
	} else {
		log.Printf("test user already exists, skipped")
	}

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
