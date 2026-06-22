package middleware

import (
	"io"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func init() {
	os.Setenv("JWT_SECRET", "test-secret")
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
