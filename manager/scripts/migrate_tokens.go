//go:build migrate_tokens

package main

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

func main() {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}

	dbPath := filepath.Join(dataDir, "manager.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// Find all users without a client_token
	var tokenlessUsers []model.User
	db.Where("client_token IS NULL OR client_token = ''").Find(&tokenlessUsers)

	if len(tokenlessUsers) == 0 {
		fmt.Println("All users already have a client_token — nothing to migrate.")
		return
	}

	fmt.Printf("Found %d user(s) without client_token\n", len(tokenlessUsers))

	for _, u := range tokenlessUsers {
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			log.Printf("ERROR: failed to generate token for user %d: %v", u.ID, err)
			continue
		}
		token := "rf_" + hex.EncodeToString(tokenBytes)
		if err := db.Model(&u).Update("client_token", token).Error; err != nil {
			log.Printf("ERROR: failed to update client_token for user %d: %v", u.ID, err)
			continue
		}
		fmt.Printf("  user %d (%s) → %s\n", u.ID, u.Username, token)
	}

	fmt.Println("Migration complete.")
}
