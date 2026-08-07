package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/middleware"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	ResetRegisterLimits()
	os.Setenv("CAPTCHA_DISABLED", "1")
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

	auth := v1.Group("/auth")
	auth.Get("/csrf", GetCSRFToken)
	auth.Get("/validate", middleware.WebAuth("session"), ValidateSession)
	auth.Post("/refresh", middleware.WebAuth("session"), Refresh)
	auth.Post("/logout", middleware.WebAuth("session"), Logout)

	adminAuth := v1.Group("/admin/auth")
	adminAuth.Post("/login", AdminLogin)
	adminAuth.Post("/logout", AdminLogout)
	adminAuth.Get("/csrf", GetCSRFToken)

	web := v1.Group("/web", middleware.WebAuth("session"))
	web.Get("/client-token", GetClientToken)
	web.Post("/client-token/regenerate", middleware.WebCSRF("csrf"), RegenerateClientToken)

	user := v1.Group("/user", middleware.WebAuth("session"))
	user.Get("/profile", GetProfile)
	user.Put("/profile", middleware.WebCSRF("csrf"), UpdateProfile)
	user.Post("/orders", middleware.WebCSRF("csrf"), CreateOrder)
	user.Get("/orders", ListOrders)

	admin := v1.Group("/admin", middleware.WebAuth("admin_session"), middleware.AdminOnly())
	admin.Get("/users", ListUsers)
	admin.Put("/users/:id", middleware.WebCSRF("admin_csrf"), UpdateUser)

	return app
}

// registerAndBrowserLogin creates a user and logs in via browser mode (no
// X-Client header), returning the login response with its Set-Cookie headers.
func registerAndBrowserLogin(t *testing.T, app *fiber.App, username, password string) *http.Response {
	t.Helper()
	regBody := fmt.Sprintf(`{"username":%q,"password":%q}`, username, password)
	req := httptest.NewRequest("POST", "/api/v1/public/register", bytes.NewReader([]byte(regBody)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("register expected 201, got %d", resp.StatusCode)
	}

	loginBody := fmt.Sprintf(`{"username":%q,"password":%q}`, username, password)
	req2 := httptest.NewRequest("POST", "/api/v1/public/login", bytes.NewReader([]byte(loginBody)))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if resp2.StatusCode != 200 {
		t.Fatalf("login expected 200, got %d", resp2.StatusCode)
	}
	return resp2
}

func getCookieByName(resp *http.Response, name string) *http.Cookie {
	for _, ck := range resp.Cookies() {
		if ck.Name == name {
			return ck
		}
	}
	return nil
}

func TestRegister_Success(t *testing.T) {
	setupTestDB(t)
	app := setupTestApp()

	body := `{"username":"reguser","password":"test123456"}`
	req := httptest.NewRequest("POST", "/api/v1/public/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client", "flutter")
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
	if authResp.User["username"] != "reguser" {
		t.Errorf("expected username reguser, got %v", authResp.User["username"])
	}
	if authResp.User["role"] != "user" {
		t.Errorf("expected role user, got %v", authResp.User["role"])
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

func TestRegister_AdminRole(t *testing.T) {
	setupTestDB(t)
	app := setupTestApp()

	// Sending is_admin in the payload must NOT escalate privileges.
	// Registration always creates a regular user; admins are seeded by db.Init.
	body := `{"username":"newadmin","password":"test123456","is_admin":true}`
	req := httptest.NewRequest("POST", "/api/v1/public/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client", "flutter")
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
	if authResp.User["role"] != "user" {
		t.Errorf("expected role user (privilege escalation blocked), got %v", authResp.User["role"])
	}
}

func TestRegister_NonAdminDefault(t *testing.T) {
	setupTestDB(t)
	app := setupTestApp()

	body := `{"username":"reguser2","password":"test123456"}`
	req := httptest.NewRequest("POST", "/api/v1/public/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client", "flutter")
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
	if authResp.User["role"] != "user" {
		t.Errorf("expected role user, got %v", authResp.User["role"])
	}
}

func TestRegister_AdminDefaultFieldIsFalse(t *testing.T) {
	setupTestDB(t)
	app := setupTestApp()

	body := `{"username":"reguser3","password":"test123456","is_admin":false}`
	req := httptest.NewRequest("POST", "/api/v1/public/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client", "flutter")
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
	if authResp.User["role"] != "user" {
		t.Errorf("expected role user, got %v", authResp.User["role"])
	}
}

func TestHealthEndpoint(t *testing.T) {
	app := fiber.New()
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "manager-api",
		})
	})

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
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
	req2.Header.Set("X-Client", "flutter")
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

func TestLogin_BrowserModeSetsCookies(t *testing.T) {
	setupTestDB(t)
	app := setupTestApp()

	resp := registerAndBrowserLogin(t, app, "webuser", "test123456")

	if getCookieByName(resp, "session") == nil {
		t.Error("browser login should set session cookie")
	}
	if getCookieByName(resp, "refresh") == nil {
		t.Error("browser login should set refresh cookie")
	}
	if getCookieByName(resp, "csrf") == nil {
		t.Error("browser login should set csrf cookie")
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to parse login response: %v", err)
	}
	if _, hasToken := body["token"]; hasToken {
		t.Error("browser login must not return a token")
	}
	userMap, ok := body["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected user object, got %T", body["user"])
	}
	if _, hasUUID := userMap["vless_uuid"]; hasUUID {
		t.Error("login user must not expose vless_uuid")
	}
}

func TestValidateSession_WithAndWithoutCookie(t *testing.T) {
	setupTestDB(t)
	app := setupTestApp()

	resp := registerAndBrowserLogin(t, app, "sessionuser", "test123456")
	session := getCookieByName(resp, "session")
	if session == nil {
		t.Fatal("no session cookie set")
	}

	// with session cookie → 200
	req := httptest.NewRequest("GET", "/api/v1/auth/validate", nil)
	req.AddCookie(session)
	resp2, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if resp2.StatusCode != 200 {
		t.Fatalf("expected 200 with session cookie, got %d", resp2.StatusCode)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(resp2.Body).Decode(&body); err != nil {
		t.Fatalf("failed to parse validate response: %v", err)
	}
	if _, ok := body["user"]; !ok {
		t.Error("expected user in validate response")
	}

	// without cookie → 401
	req3 := httptest.NewRequest("GET", "/api/v1/auth/validate", nil)
	resp3, err := app.Test(req3, -1)
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if resp3.StatusCode != 401 {
		t.Fatalf("expected 401 without session cookie, got %d", resp3.StatusCode)
	}
}

func TestWebCSRF_RegenerateClientToken(t *testing.T) {
	setupTestDB(t)
	app := setupTestApp()

	resp := registerAndBrowserLogin(t, app, "csrfuser", "test123456")
	session := getCookieByName(resp, "session")
	csrf := getCookieByName(resp, "csrf")
	if session == nil || csrf == nil {
		t.Fatal("login must set session and csrf cookies")
	}

	// missing X-CSRF-Token → 403
	req := httptest.NewRequest("POST", "/api/v1/web/client-token/regenerate", nil)
	req.AddCookie(session)
	req.AddCookie(csrf)
	resp2, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("regenerate failed: %v", err)
	}
	if resp2.StatusCode != 403 {
		t.Fatalf("expected 403 without X-CSRF-Token, got %d", resp2.StatusCode)
	}

	// correct X-CSRF-Token → pass
	req3 := httptest.NewRequest("POST", "/api/v1/web/client-token/regenerate", nil)
	req3.AddCookie(session)
	req3.AddCookie(csrf)
	req3.Header.Set("X-CSRF-Token", csrf.Value)
	resp3, err := app.Test(req3, -1)
	if err != nil {
		t.Fatalf("regenerate failed: %v", err)
	}
	if resp3.StatusCode != 200 {
		t.Fatalf("expected 200 with correct X-CSRF-Token, got %d", resp3.StatusCode)
	}
}

func TestAdminLogin_NonAdminForbidden(t *testing.T) {
	setupTestDB(t)
	app := setupTestApp()

	registerAndBrowserLogin(t, app, "regularguy", "test123456")

	loginBody := `{"username":"regularguy","password":"test123456"}`
	req := httptest.NewRequest("POST", "/api/v1/admin/auth/login", bytes.NewReader([]byte(loginBody)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("admin login failed: %v", err)
	}
	if resp.StatusCode != 403 {
		t.Fatalf("expected 403 for non-admin login, got %d", resp.StatusCode)
	}
}

func TestAdminLogin_Success(t *testing.T) {
	setupTestDB(t)
	app := setupTestApp()

	hash, err := bcrypt.GenerateFromPassword([]byte("test123456"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	admin := model.User{
		Username:     "rootadmin",
		PasswordHash: string(hash),
		Role:         "admin",
		Status:       "active",
	}
	if err := db.DB.Create(&admin).Error; err != nil {
		t.Fatalf("failed to create admin: %v", err)
	}

	loginBody := `{"username":"rootadmin","password":"test123456"}`
	req := httptest.NewRequest("POST", "/api/v1/admin/auth/login", bytes.NewReader([]byte(loginBody)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("admin login failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if getCookieByName(resp, "admin_session") == nil {
		t.Error("admin login should set admin_session cookie")
	}
	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to parse admin login response: %v", err)
	}
	if body["role"] != "admin" {
		t.Errorf("expected role admin, got %v", body["role"])
	}
}

func TestGetProfile_NoVlessUUID(t *testing.T) {
	setupTestDB(t)
	app := setupTestApp()

	resp := registerAndBrowserLogin(t, app, "profileuser", "test123456")
	session := getCookieByName(resp, "session")
	if session == nil {
		t.Fatal("no session cookie set")
	}

	req := httptest.NewRequest("GET", "/api/v1/user/profile", nil)
	req.AddCookie(session)
	resp2, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("profile failed: %v", err)
	}
	if resp2.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
	bodyBytes, err := io.ReadAll(resp2.Body)
	if err != nil {
		t.Fatalf("failed to read profile body: %v", err)
	}
	if strings.Contains(string(bodyBytes), "vless_uuid") {
		t.Error("profile must not expose vless_uuid")
	}
	if strings.Contains(string(bodyBytes), "password_hash") {
		t.Error("profile must not expose password_hash")
	}
}
