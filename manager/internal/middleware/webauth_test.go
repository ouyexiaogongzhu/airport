package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func TestWebAuth_NoCookie(t *testing.T) {
	app := fiber.New()
	app.Get("/protected", WebAuth("session"), func(c *fiber.Ctx) error {
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
	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"error":"SESSION_EXPIRED"}` {
		t.Fatalf("expected SESSION_EXPIRED body, got %s", string(body))
	}
}

func TestWebAuth_ExpiredCookie(t *testing.T) {
	app := fiber.New()
	app.Get("/protected", WebAuth("session"), func(c *fiber.Ctx) error {
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
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.StatusCode)
	}
}

func TestWebAuth_ValidCookieSetsLocals(t *testing.T) {
	app := fiber.New()
	app.Get("/protected", WebAuth("session"), func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"user_id":  c.Locals("user_id"),
			"username": c.Locals("username"),
			"role":     c.Locals("role"),
		})
	})

	token := generateTestToken(jwt.MapClaims{
		"user_id":  float64(42),
		"username": "alice",
		"role":     "admin",
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	want := `{"role":"admin","user_id":42,"username":"alice"}`
	if string(body) != want {
		t.Fatalf("expected body %s, got %s", want, string(body))
	}
}

// A Flutter-style Bearer request must never authenticate on cookie routes.
func TestWebAuth_RejectsFlutterBearer(t *testing.T) {
	app := fiber.New()
	app.Get("/protected", WebAuth("session"), func(c *fiber.Ctx) error {
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
	req.Header.Set("X-Client", "flutter")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected status 401 for Bearer without cookie, got %d", resp.StatusCode)
	}
}

func TestWebCSRF_MissingHeader(t *testing.T) {
	app := fiber.New()
	app.Post("/mutate", WebCSRF("csrf"), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("POST", "/mutate", nil)
	req.AddCookie(&http.Cookie{Name: "csrf", Value: "abc123"})
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected status 403, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"error":"CSRF_INVALID"}` {
		t.Fatalf("expected CSRF_INVALID body, got %s", string(body))
	}
}

func TestWebCSRF_WrongHeader(t *testing.T) {
	app := fiber.New()
	app.Post("/mutate", WebCSRF("csrf"), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("POST", "/mutate", nil)
	req.AddCookie(&http.Cookie{Name: "csrf", Value: "abc123"})
	req.Header.Set("X-CSRF-Token", "wrong")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected status 403, got %d", resp.StatusCode)
	}
}

func TestWebCSRF_ValidHeader(t *testing.T) {
	app := fiber.New()
	app.Post("/mutate", WebCSRF("csrf"), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("POST", "/mutate", nil)
	req.AddCookie(&http.Cookie{Name: "csrf", Value: "abc123"})
	req.Header.Set("X-CSRF-Token", "abc123")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestWebCSRF_SafeMethodsSkipped(t *testing.T) {
	app := fiber.New()
	app.Get("/read", WebCSRF("csrf"), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/read", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected GET to skip CSRF, got %d", resp.StatusCode)
	}
}

func TestWebCSRF_WrongCookieName(t *testing.T) {
	app := fiber.New()
	app.Post("/mutate", WebCSRF("admin_csrf"), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Header matches the portal csrf cookie but the middleware reads admin_csrf.
	req := httptest.NewRequest("POST", "/mutate", nil)
	req.AddCookie(&http.Cookie{Name: "csrf", Value: "abc123"})
	req.Header.Set("X-CSRF-Token", "abc123")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected status 403, got %d", resp.StatusCode)
	}
}
