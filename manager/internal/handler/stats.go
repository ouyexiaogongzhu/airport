package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
)

// AdminStats aggregates key metrics for the admin dashboard.
type AdminStats struct {
	TotalUsers      int64          `json:"total_users"`
	ActiveUsers     int64          `json:"active_users"`
	ActiveOrders    int64          `json:"active_orders"`
	TotalProducts   int64          `json:"total_products"`
	RevenueMTD      float64        `json:"revenue_mtd"`
	TotalNodes      int64          `json:"total_nodes"`
	OnlineNodes     int64          `json:"online_nodes"`
	NodeTrafficUp   int64          `json:"node_traffic_up"`
	NodeTrafficDown int64          `json:"node_traffic_down"`
	TrafficTrend    []TrafficPoint `json:"traffic_trend"`
	RecentOrders    []orderWithUser `json:"recent_orders"`
}

// TrafficPoint is one bucket of the traffic trend chart.
type TrafficPoint struct {
	Day     string `json:"day"`
	Upload  int64  `json:"upload"`
	Download int64 `json:"download"`
}

// orderWithUser is an order joined with its owning user's username.
type orderWithUser struct {
	model.Order
	Username string `json:"username"`
}

// GetAdminStats returns aggregate dashboard metrics.
// GET /api/v1/admin/stats
func GetAdminStats(c *fiber.Ctx) error {
	stats := AdminStats{}

	// Users
	db.DB.Model(&model.User{}).Count(&stats.TotalUsers)
	db.DB.Model(&model.User{}).Where("subscription_status = ? AND (expire_time = 0 OR expire_time > ?)", "active", time.Now().Unix()).Count(&stats.ActiveUsers)

	// Orders: count paid orders this month for revenue, and total active (paid) orders.
	db.DB.Model(&model.Order{}).Where("status = ?", "paid").Count(&stats.ActiveOrders)
	monthStart := time.Now().Truncate(24*time.Hour).AddDate(0, 0, -time.Now().Day()+1)
	db.DB.Model(&model.Order{}).Where("status = ? AND created_at >= ?", "paid", monthStart).
		Select("COALESCE(SUM(amount), 0)").Scan(&stats.RevenueMTD)

	// Products
	db.DB.Model(&model.Product{}).Count(&stats.TotalProducts)

	// Nodes: total, online (last heartbeat within 5 minutes), traffic.
	db.DB.Model(&model.Node{}).Count(&stats.TotalNodes)
	cutoff := time.Now().Add(-5 * time.Minute)
	db.DB.Model(&model.Node{}).Where("last_heartbeat >= ? OR status = ?", cutoff, "active").Count(&stats.OnlineNodes)
	db.DB.Model(&model.Node{}).Select("COALESCE(SUM(traffic_up), 0), COALESCE(SUM(traffic_down), 0)").
		Row().Scan(&stats.NodeTrafficUp, &stats.NodeTrafficDown)

	// Traffic trend: last 7 days, bucketed by day.
	stats.TrafficTrend = loadTrafficTrend()

	// Recent 5 orders with usernames.
	stats.RecentOrders = loadRecentOrders(5)

	return c.JSON(stats)
}

func loadTrafficTrend() []TrafficPoint {
	points := make([]TrafficPoint, 0, 7)
	start := time.Now().Truncate(24*time.Hour).AddDate(0, 0, -6)

	for i := 0; i < 7; i++ {
		dayStart := start.AddDate(0, 0, i)
		points = append(points, TrafficPoint{Day: dayStart.Format("01-02")})
	}

	// Aggregate per day.
	type row struct {
		Day      string
		Upload   int64
		Download int64
	}
	var rows []row
	db.DB.Model(&model.TrafficRecord{}).
		Select("strftime('%m-%d', recorded_at) AS day, SUM(upload_bytes) AS upload, SUM(download_bytes) AS download").
		Where("recorded_at >= ?", start).
		Group("strftime('%m-%d', recorded_at)").
		Scan(&rows)

	byDay := make(map[string]*TrafficPoint, len(rows))
	for i := range points {
		byDay[points[i].Day] = &points[i]
	}
	for _, r := range rows {
		if p, ok := byDay[r.Day]; ok {
			p.Upload = r.Upload
			p.Download = r.Download
		}
	}
	return points
}

func loadRecentOrders(limit int) []orderWithUser {
	var orders []orderWithUser
	db.DB.Model(&model.Order{}).
		Select("orders.*, users.username").
		Joins("LEFT JOIN users ON users.id = orders.user_id").
		Order("orders.created_at DESC").
		Limit(limit).
		Scan(&orders)
	return orders
}
