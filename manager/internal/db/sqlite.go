package db

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
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

	// Auto migrate
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

	log.Printf("database initialized at %s", dbPath)
}
