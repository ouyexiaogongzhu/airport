package db

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init(dataDir string) {
	if dataDir == "" {
		dataDir = "data"
	}
	// Ensure directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("failed to create data dir: %v", err)
	}

	dbPath := filepath.Join(dataDir, "manager.db")
	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// Auto migrate.
	// TODO: Replace AutoMigrate with a proper migration tool (e.g. golang-migrate/migrate)
	// for production use, to support versioned, reversible schema changes.

	if err := DB.AutoMigrate(
		&model.User{},
		&model.Order{},
		&model.Product{},
		&model.Node{},
		&model.TrafficRecord{},
	); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	// Backfill client_token for existing users
	var tokenlessUsers []model.User
	DB.Where("client_token IS NULL OR client_token = ''").Find(&tokenlessUsers)
	for _, u := range tokenlessUsers {
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			continue
		}
		token := "rf_" + hex.EncodeToString(tokenBytes)
		DB.Model(&u).Update("client_token", token)
		fmt.Printf("backfilled client_token for user %d\n", u.ID)
	}

	// Auto-create admin user if no users exist
	var count int64
	DB.Model(&model.User{}).Count(&count)
	if count == 0 {
		// Generate random admin password
		passBytes := make([]byte, 16)
		if _, err := rand.Read(passBytes); err != nil {
			log.Fatalf("failed to generate admin password: %v", err)
		}
		adminPass := hex.EncodeToString(passBytes)

		adminHash, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
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
		if result := DB.Create(&admin); result.Error != nil {
			log.Fatalf("failed to create admin user: %v", result.Error)
		}
		// Generate client_token for admin
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err == nil {
			admin.ClientToken = "rf_" + hex.EncodeToString(tokenBytes)
			DB.Model(&admin).Update("client_token", admin.ClientToken)
		}
		log.Printf("========================================")
		log.Printf("Admin user created successfully!")
		log.Printf("  Username: admin")
		if len(adminPass) > 4 {
			log.Printf("  Password: %s****", adminPass[:4])
		} else {
			log.Printf("  Password: ****")
		}
		log.Printf("  Full password written to: %s/.admin_password", dataDir)
		log.Printf("  PLEASE CHANGE THIS PASSWORD IMMEDIATELY")
		log.Printf("========================================")
		// Write full password to file for one-time retrieval
		pwFile := filepath.Join(dataDir, ".admin_password")
		if err := os.WriteFile(pwFile, []byte(adminPass), 0600); err != nil {
			log.Printf("WARNING: failed to write admin password file: %v", err)
		}
	}

	log.Printf("database initialized at %s", dbPath)
}
