package handler

import (
	"bytes"
	"encoding/json"
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

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := t.TempDir() + "/test.db"
	database, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect test DB: %v", err)
	}
	if err := database.AutoMigrate(&model.User{}, &model.Order{}, &model.Product{}, &model.Node{}, &model.TrafficRecord{}); err != nil {
		t.Fatalf("failed to migrate test DB: %v", err)
	}
	db.DB = database
	os.Setenv("JWT_SECRET", "test-secret")
	return database
}

func generateTestToken(user *model.User) string {
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     user.Role,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte("test-secret"))
	return signed
}

func setupTestApp() *fiber.App {
	app := fiber.New()
	v1 := app.Group("/api/v1")
	public := v1.Group("/public")
	public.Post("/register", Register)
	public.Post("/login", Login)
	public.Post("/payment/callback", MockPayCallback)
	return app
}

func TestRegister_Success(t *testing.T) {
	setupTestDB(t)
	app := setupTestApp()

	body := `{"username":"reguser","password":"test123456"}`
	req := httptest.NewRequest("POST", "/api/v1/public/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var authResp AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if authResp.Token == "" {
		t.Error("expected non-empty token")
	}
	if authResp.User.Username != "reguser" {
		t.Errorf("expected username reguser, got %s", authResp.User.Username)
	}
	if authResp.User.Role != "user" {
		t.Errorf("expected role user, got %s", authResp.User.Role)
	}
}

func TestRegister_DuplicateUsername(t *testing.T) {
	setupTestDB(t)
	app := setupTestApp()

	body := `{"username":"dupuser","password":"test123456"}`
	req := httptest.NewRequest("POST", "/api/v1/public/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("first register failed: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("expected 201 on first register, got %d", resp.StatusCode)
	}

	req2 := httptest.NewRequest("POST", "/api/v1/public/register", bytes.NewReader([]byte(body)))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("second register failed: %v", err)
	}
	if resp2.StatusCode != 409 {
		t.Fatalf("expected 409 for duplicate, got %d", resp2.StatusCode)
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	setupTestDB(t)
	app := setupTestApp()

	body := `{"username":"shortuser","password":"ab"}`
	req := httptest.NewRequest("POST", "/api/v1/public/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for short password, got %d", resp.StatusCode)
	}
}

func TestLogin_Success(t *testing.T) {
	setupTestDB(t)
	app := setupTestApp()

	// First register
	regBody := `{"username":"loginuser","password":"test123456"}`
	req := httptest.NewRequest("POST", "/api/v1/public/register", bytes.NewReader([]byte(regBody)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("register expected 201, got %d", resp.StatusCode)
	}

	// Then login
	loginBody := `{"username":"loginuser","password":"test123456"}`
	req2 := httptest.NewRequest("POST", "/api/v1/public/login", bytes.NewReader([]byte(loginBody)))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if resp2.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
	var authResp AuthResponse
	if err := json.NewDecoder(resp2.Body).Decode(&authResp); err != nil {
		t.Fatalf("failed to parse login response: %v", err)
	}
	if authResp.Token == "" {
		t.Error("expected non-empty token from login")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	setupTestDB(t)
	app := setupTestApp()

	// First register
	regBody := `{"username":"loginuser2","password":"test123456"}`
	req := httptest.NewRequest("POST", "/api/v1/public/register", bytes.NewReader([]byte(regBody)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("register expected 201, got %d", resp.StatusCode)
	}

	// Login with wrong password
	loginBody := `{"username":"loginuser2","password":"wrongpassword"}`
	req2 := httptest.NewRequest("POST", "/api/v1/public/login", bytes.NewReader([]byte(loginBody)))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if resp2.StatusCode != 401 {
		t.Fatalf("expected 401 for wrong password, got %d", resp2.StatusCode)
	}
}

func TestLogin_NonexistentUser(t *testing.T) {
	setupTestDB(t)
	app := setupTestApp()

	body := `{"username":"nobody","password":"test123456"}`
	req := httptest.NewRequest("POST", "/api/v1/public/login", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401 for nonexistent user, got %d", resp.StatusCode)
	}
}
