package middleware

import (
	"io"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func init() {
	os.Setenv("JWT_SECRET", "test-secret")
}

// setupTestDB points db.DB at an in-memory test database.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	ResetNodeTokenCache()
	database, err := gorm.Open(sqlite.Open(t.TempDir()+"/test.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect test DB: %v", err)
	}
	if err := database.AutoMigrate(&model.User{}, &model.Node{}); err != nil {
		t.Fatalf("failed to migrate test DB: %v", err)
	}
	db.DB = database
	return database
}

func createTestNode(t *testing.T, token string) *model.Node {
	t.Helper()
	node := model.Node{
		Name:     "test-node",
		Type:     "xray",
		Address:  "1.2.3.4",
		Port:     443,
		Protocol: "vless",
		Status:   "active",
		Token:    token,
	}
	if err := db.DB.Create(&node).Error; err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	return &node
}

func generateTestToken(claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString(getJWTSecret())
	return signed
}

func TestJWTProtected_NoToken(t *testing.T) {
	app := fiber.New()
	app.Get("/protected", JWTProtected(), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.StatusCode)
	}
}

func TestJWTProtected_InvalidToken(t *testing.T) {
	app := fiber.New()
	app.Get("/protected", JWTProtected(), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalidtoken")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.StatusCode)
	}
}

func TestJWTProtected_ExpiredToken(t *testing.T) {
	app := fiber.New()
	app.Get("/protected", JWTProtected(), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	token := generateTestToken(jwt.MapClaims{
		"user_id":  float64(1),
		"username": "test",
		"role":     "user",
		"exp":      time.Now().Add(-1 * time.Hour).Unix(),
		"iat":      time.Now().Add(-2 * time.Hour).Unix(),
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.StatusCode)
	}
}

func TestJWTProtected_Success(t *testing.T) {
	app := fiber.New()
	app.Get("/protected", JWTProtected(), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	token := generateTestToken(jwt.MapClaims{
		"user_id":  float64(1),
		"username": "test",
		"role":     "user",
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if bodyBytes, _ := io.ReadAll(resp.Body); string(bodyBytes) != "ok" {
		t.Fatalf("expected body ok, got %s", string(bodyBytes))
	}
}

func TestAdminOnly_NonAdmin(t *testing.T) {
	app := fiber.New()
	app.Get("/admin", JWTProtected(), AdminOnly(), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	token := generateTestToken(jwt.MapClaims{
		"user_id":  float64(1),
		"username": "test",
		"role":     "user",
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	})

	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected status 403, got %d", resp.StatusCode)
	}
}

func TestAdminOnly_Admin(t *testing.T) {
	app := fiber.New()
	app.Get("/admin", JWTProtected(), AdminOnly(), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	token := generateTestToken(jwt.MapClaims{
		"user_id":  float64(1),
		"username": "admin",
		"role":     "admin",
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	})

	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if bodyBytes, _ := io.ReadAll(resp.Body); string(bodyBytes) != "ok" {
		t.Fatalf("expected body ok, got %s", string(bodyBytes))
	}
}
