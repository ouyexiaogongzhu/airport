package handler

import (
	"strconv"
	"testing"
	"time"

	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openBenchDB wires db.DB to a fresh on-disk sqlite DB for benchmarks. Run
// benchmarks with `-run ^$` so they do not share the process with unit tests.
func openBenchDB(b *testing.B) *gorm.DB {
	b.Helper()
	database, err := gorm.Open(sqlite.Open(b.TempDir()+"/bench.db"), &gorm.Config{})
	if err != nil {
		b.Fatalf("failed to connect bench DB: %v", err)
	}
	if err := database.AutoMigrate(&model.User{}, &model.Node{}, &model.TrafficRecord{}); err != nil {
		b.Fatalf("failed to migrate bench DB: %v", err)
	}
	db.DB = database
	return database
}

func benchSetupTrafficUsers(b *testing.B) {
	b.Helper()
	openBenchDB(b)
	for i := 1; i <= 10; i++ {
		u := model.User{
			Username:    "bench_traffic_u" + strconv.Itoa(i),
			Status:      "active",
			ClientToken: "bt_" + strconv.Itoa(i),
		}
		if err := db.DB.Create(&u).Error; err != nil {
			b.Fatalf("failed to create bench user: %v", err)
		}
	}
}

// BenchmarkTrafficWrite_Batch100 models ReportNodeTraffic after the N+1 fix: a
// single multi-row INSERT for 100 records plus one aggregated UPDATE per user.
func BenchmarkTrafficWrite_Batch100(b *testing.B) {
	benchSetupTrafficUsers(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		records := make([]model.TrafficRecord, 0, 100)
		for j := 0; j < 100; j++ {
			records = append(records, model.TrafficRecord{
				NodeID:       1,
				UserID:       uint(j%10 + 1),
				UploadBytes:  10,
				DownloadBytes: 20,
				RecordedAt:   time.Now(),
			})
		}
		if err := db.DB.Create(&records).Error; err != nil {
			b.Fatalf("batch insert failed: %v", err)
		}
		for u := uint(1); u <= 10; u++ {
			db.DB.Model(&model.User{}).Where("id = ?", u).
				Update("traffic_used_bytes", gorm.Expr("traffic_used_bytes + ?", 300))
		}
		db.DB.Exec("DELETE FROM traffic_records")
	}
}

// BenchmarkTrafficWrite_Legacy100 models the pre-optimisation ReportNodeTraffic:
// one INSERT plus one user UPDATE per entry.
func BenchmarkTrafficWrite_Legacy100(b *testing.B) {
	benchSetupTrafficUsers(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			rec := model.TrafficRecord{
				NodeID:       1,
				UserID:       uint(j%10 + 1),
				UploadBytes:  10,
				DownloadBytes: 20,
				RecordedAt:   time.Now(),
			}
			if err := db.DB.Create(&rec).Error; err != nil {
				b.Fatalf("insert failed: %v", err)
			}
			db.DB.Model(&model.User{}).Where("id = ?", rec.UserID).
				Update("traffic_used_bytes", gorm.Expr("traffic_used_bytes + ?", 30))
		}
		db.DB.Exec("DELETE FROM traffic_records")
	}
}

func benchSetupConfigUsers(b *testing.B) *model.Node {
	b.Helper()
	openBenchDB(b)
	for i := 0; i < 100; i++ {
		u := model.User{
			Username:           "bench_cfg_" + strconv.Itoa(i),
			ClientToken:        "bc_" + strconv.Itoa(i),
			SubscriptionStatus: "active",
			ExpireTime:         time.Now().Add(24 * time.Hour).Unix(),
		}
		ensureUserCredentials(&u)
		if err := db.DB.Create(&u).Error; err != nil {
			b.Fatalf("failed to create bench user: %v", err)
		}
	}
	node := model.Node{
		Name:     "BenchNode",
		Type:     "xray",
		Address:  "1.2.3.4",
		Port:     443,
		Protocol: "vless",
		Status:   "active",
		UserID:   1,
	}
	if err := db.DB.Create(&node).Error; err != nil {
		b.Fatalf("failed to create bench node: %v", err)
	}
	return &node
}

// BenchmarkBuildNodeXrayConfig_CacheHit measures the steady-state daemon pull:
// only the cheap id-fingerprint query runs, no user rows are loaded.
func BenchmarkBuildNodeXrayConfig_CacheHit(b *testing.B) {
	node := benchSetupConfigUsers(b)
	if _, err := BuildNodeXrayConfig(node, db.DB); err != nil {
		b.Fatalf("warm-up build failed: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := BuildNodeXrayConfig(node, db.DB); err != nil {
			b.Fatalf("cached build failed: %v", err)
		}
	}
}

// BenchmarkBuildNodeXrayConfig_CacheMiss measures a full rebuild (e.g. right
// after a user change): fingerprint + full user load + config assembly.
func BenchmarkBuildNodeXrayConfig_CacheMiss(b *testing.B) {
	node := benchSetupConfigUsers(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ResetNodeConfigCache()
		if _, err := BuildNodeXrayConfig(node, db.DB); err != nil {
			b.Fatalf("rebuild failed: %v", err)
		}
	}
}
