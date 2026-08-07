package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	mathrand "math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
	"golang.org/x/crypto/bcrypt"
)

var registerLimits sync.Map

type captchaEntry struct {
	answer    string
	expiresAt time.Time
}

var captchaStore sync.Map

// TODO: The cleanup goroutine started by captchaCleanupOnce is never stopped.
// It leaks for the lifetime of the process. Consider using a context-aware
// ticker or accepting a stop channel so the goroutine can be shut down gracefully.
var captchaCleanupOnce sync.Once

func storeCaptcha(token, answer string) {
	captchaStore.Store(token, &captchaEntry{
		answer:    answer,
		expiresAt: time.Now().Add(5 * time.Minute),
	})
}

func verifyCaptcha(token, answer string) bool {
	val, ok := captchaStore.LoadAndDelete(token)
	if !ok {
		return false
	}
	entry := val.(*captchaEntry)
	if time.Now().After(entry.expiresAt) {
		return false
	}
	return entry.answer == answer
}

func generateCaptchaToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		log.Printf("[ERROR] failed to generate captcha token: %v", err)
		return ""
	}
	return hex.EncodeToString(b)
}

// ResetRegisterLimits clears the per-IP registration counters (used in tests).
func ResetRegisterLimits() {
	registerLimits.Range(func(key, value interface{}) bool {
		registerLimits.Delete(key)
		return true
	})
}

type registerCounter struct {
	mu         sync.Mutex
	timestamps []time.Time
}

func checkRegisterLimit(ip string) bool {
	const maxAttempts = 5
	const window = time.Hour

	now := time.Now()
	val, _ := registerLimits.LoadOrStore(ip, &registerCounter{})
	rc := val.(*registerCounter)

	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Prune timestamps outside the sliding window
	var valid []time.Time
	for _, t := range rc.timestamps {
		if now.Sub(t) < window {
			valid = append(valid, t)
		}
	}
	rc.timestamps = valid

	if len(rc.timestamps) >= maxAttempts {
		return false
	}

	rc.timestamps = append(rc.timestamps, now)
	return true
}

type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=64"`
	Password string `json:"password" validate:"required,min=8"`
}

type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type TokenLoginRequest struct {
	Token string `json:"token"`
}

type AuthResponse struct {
	Token string                 `json:"token"`
	User  map[string]interface{} `json:"user"`
}

func getJWTSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatalf("[FATAL] JWT_SECRET environment variable is required (set it before starting the server)")
	}
	if len(secret) < 16 {
		log.Printf("[WARNING] JWT_SECRET is shorter than 16 characters; use a long random string in production")
	}
	return []byte(secret)
}

// Register creates a new user account.
func Register(c *fiber.Ctx) error {
	req := new(RegisterRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if !checkRegisterLimit(c.IP()) {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error": "registration rate limit exceeded",
		})
	}

	if req.Username == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "username and password are required",
		})
	}

	if len(req.Password) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "password must be at least 8 characters",
		})
	}

	// Check existing user
	var existing model.User
	if result := db.DB.Where("username = ?", req.Username).First(&existing); result.Error == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "username already exists",
		})
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to hash password",
		})
	}

	user := model.User{
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         "user",
		Status:       "active",
		Balance:      0,
	}

	// Generate per-user proxy credentials and client token
	ensureUserCredentials(&user)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate client token",
		})
	}
	user.ClientToken = "rf_" + hex.EncodeToString(tokenBytes)

	if result := db.DB.Create(&user); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create user",
		})
	}

	// Flutter keeps the Bearer JWT flow; browsers get httpOnly session cookies.
	if isFlutter(c) {
		token, err := generateToken(&user)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to generate token",
			})
		}
		return c.Status(fiber.StatusCreated).JSON(AuthResponse{
			Token: token,
			User:  SanitizedUser(&user),
		})
	}

	if err := setWebAuthCookies(c, &user); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to establish session",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"user": SanitizedUser(&user),
	})
}

// CaptchaEndpoint returns a math captcha challenge.
func CaptchaEndpoint(c *fiber.Ctx) error {
	captchaCleanupOnce.Do(func() {
		// NOTE: This goroutine runs for the lifetime of the process with no stop channel.
		// Acceptable for a long-running server; would need a context-based shutdown for tests.
		go func() {
			ticker := time.NewTicker(1 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				now := time.Now()
				captchaStore.Range(func(key, val interface{}) bool {
					entry := val.(*captchaEntry)
					if now.After(entry.expiresAt) {
						captchaStore.Delete(key)
					}
					return true
				})
			}
		}()
	})

	a := mathrand.Intn(10) + 1
	b := mathrand.Intn(10) + 1
	answer := strconv.Itoa(a + b)
	question := fmt.Sprintf("%d + %d = ?", a, b)
	token := generateCaptchaToken()

	storeCaptcha(token, answer)

	return c.JSON(fiber.Map{
		"question": question,
		"token":    token,
	})
}

// Login authenticates a user and returns a JWT.
func Login(c *fiber.Ctx) error {
	req := new(LoginRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.Username == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "username and password are required",
		})
	}

	var user model.User
	if result := db.DB.Where("username = ?", req.Username).First(&user); result.Error != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid username or password",
		})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid username or password",
		})
	}

	if user.Status != "active" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "account is not active",
		})
	}

	// Flutter keeps the Bearer JWT flow; browsers get httpOnly session cookies.
	if isFlutter(c) {
		token, err := generateToken(&user)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to generate token",
			})
		}
		return c.JSON(AuthResponse{
			Token: token,
			User:  SanitizedUser(&user),
		})
	}

	if err := setWebAuthCookies(c, &user); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to establish session",
		})
	}

	return c.JSON(fiber.Map{
		"user": SanitizedUser(&user),
	})
}

// TokenLogin authenticates a user via client token and returns a JWT.
func TokenLogin(c *fiber.Ctx) error {
	req := new(TokenLoginRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}
	if req.Token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "token is required",
		})
	}

	var user model.User
	if result := db.DB.Where("client_token = ?", req.Token).First(&user); result.Error != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "INVALID_TOKEN",
		})
	}

	if user.SubscriptionStatus == "expired" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "SUBSCRIPTION_EXPIRED",
		})
	}
	if user.SubscriptionStatus == "pending" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "SUBSCRIPTION_PENDING",
		})
	}

	token, err := generateToken(&user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate token",
		})
	}

	return c.JSON(AuthResponse{
		Token: token,
		User:  SanitizedUser(&user),
	})
}

func generateToken(user *model.User) (string, error) {
	return generateTokenWithTTL(user, 24*time.Hour)
}

func generateTokenWithTTL(user *model.User, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     user.Role,
		"exp":      time.Now().Add(ttl).Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(getJWTSecret())
}

const (
	sessionCookieTTL = 30 * 24 * time.Hour
	refreshCookieTTL = 90 * 24 * time.Hour
)

// isFlutter reports whether the request comes from the Flutter client, which
// authenticates via Bearer JWT instead of httpOnly cookies.
func isFlutter(c *fiber.Ctx) bool {
	return c.Get("X-Client") == "flutter"
}

// cookieDomain returns the Domain attribute for cookies (empty for host-only,
// e.g. dev over localhost).
func cookieDomain() string {
	return os.Getenv("COOKIE_DOMAIN")
}

// setWebAuthCookies issues the portal session/refresh/csrf cookies.
func setWebAuthCookies(c *fiber.Ctx, user *model.User) error {
	if err := setSessionCookie(c, "session", user); err != nil {
		return err
	}
	if err := setRefreshCookie(c, "refresh", user); err != nil {
		return err
	}
	setCSRFCookie(c, "csrf")
	return nil
}

// setAdminAuthCookies issues the admin session/refresh/csrf cookies.
func setAdminAuthCookies(c *fiber.Ctx, user *model.User) error {
	if err := setSessionCookie(c, "admin_session", user); err != nil {
		return err
	}
	if err := setRefreshCookie(c, "admin_refresh", user); err != nil {
		return err
	}
	setCSRFCookie(c, "admin_csrf")
	return nil
}

func setSessionCookie(c *fiber.Ctx, name string, user *model.User) error {
	token, err := generateTokenWithTTL(user, sessionCookieTTL)
	if err != nil {
		return err
	}
	c.Cookie(&fiber.Cookie{
		Name:     name,
		Value:    token,
		Domain:   cookieDomain(),
		Path:     "/",
		MaxAge:   int(sessionCookieTTL / time.Second),
		Secure:   true,
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteStrictMode,
	})
	return nil
}

func setRefreshCookie(c *fiber.Ctx, name string, user *model.User) error {
	token, err := generateTokenWithTTL(user, refreshCookieTTL)
	if err != nil {
		return err
	}
	c.Cookie(&fiber.Cookie{
		Name:     name,
		Value:    token,
		Domain:   cookieDomain(),
		Path:     "/",
		MaxAge:   int(refreshCookieTTL / time.Second),
		Secure:   true,
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteStrictMode,
	})
	return nil
}

// setCSRFCookie issues a double-submit CSRF token. It is intentionally NOT
// HttpOnly so the frontend can read it via JS and echo it in X-CSRF-Token.
func setCSRFCookie(c *fiber.Ctx, name string) {
	c.Cookie(&fiber.Cookie{
		Name:     name,
		Value:    randomHex(32),
		Domain:   cookieDomain(),
		Path:     "/",
		MaxAge:   int(sessionCookieTTL / time.Second),
		Secure:   true,
		HTTPOnly: false,
		SameSite: fiber.CookieSameSiteStrictMode,
	})
}

func clearCookie(c *fiber.Ctx, name string) {
	c.Cookie(&fiber.Cookie{
		Name:     name,
		Value:    "",
		Domain:   cookieDomain(),
		Path:     "/",
		MaxAge:   -1,
		Secure:   true,
		HTTPOnly: false,
		SameSite: fiber.CookieSameSiteStrictMode,
	})
}

// SanitizedUser returns the safe subset of a user that can be exposed to web
// frontends. Credentials (password hashes, proxy secrets, vless_uuid) are
// never included; client_token is kept because the portal needs it to render
// subscription links.
func SanitizedUser(u *model.User) map[string]interface{} {
	return map[string]interface{}{
		"id":                   u.ID,
		"username":             u.Username,
		"role":                 u.Role,
		"status":               u.Status,
		"balance":              u.Balance,
		"subscription_status":  u.SubscriptionStatus,
		"subscription_tier":    u.SubscriptionTier,
		"traffic_limit_bytes":  u.TrafficLimitBytes,
		"traffic_used_bytes":   u.TrafficUsedBytes,
		"expire_time":          u.ExpireTime,
		"rate_limit_bps":       u.RateLimitBps,
		"traffic_period_start": u.TrafficPeriodStart,
		"client_token":         u.ClientToken,
		"created_at":           u.CreatedAt,
	}
}

// GetCSRFToken ensures the csrf and admin_csrf cookies exist so the portal and
// admin frontends can each read their own token via JS for the double-submit
// header.
func GetCSRFToken(c *fiber.Ctx) error {
	if c.Cookies("csrf") == "" {
		setCSRFCookie(c, "csrf")
	}
	if c.Cookies("admin_csrf") == "" {
		setCSRFCookie(c, "admin_csrf")
	}
	return c.JSON(fiber.Map{"ok": true})
}

// Logout clears all session cookies (portal and admin).
func Logout(c *fiber.Ctx) error {
	for _, name := range []string{"session", "refresh", "csrf", "admin_session", "admin_refresh", "admin_csrf"} {
		clearCookie(c, name)
	}
	return c.JSON(fiber.Map{"ok": true})
}

// AdminLogout clears the admin session cookies.
func AdminLogout(c *fiber.Ctx) error {
	for _, name := range []string{"admin_session", "admin_refresh", "admin_csrf"} {
		clearCookie(c, name)
	}
	return c.JSON(fiber.Map{"ok": true})
}

// parseJWT validates a JWT string the same way the middleware does and returns
// its claims.
func parseJWT(tokenStr string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fiber.ErrUnauthorized
		}
		return getJWTSecret(), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

// Refresh re-signs the session cookie from the refresh cookie.
func Refresh(c *fiber.Ctx) error {
	refreshToken := c.Cookies("refresh")
	if refreshToken == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "SESSION_EXPIRED"})
	}
	claims, err := parseJWT(refreshToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "SESSION_EXPIRED"})
	}
	userID, ok := claims["user_id"].(float64)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "SESSION_EXPIRED"})
	}
	var user model.User
	if result := db.DB.First(&user, uint(userID)); result.Error != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "SESSION_EXPIRED"})
	}
	if err := setSessionCookie(c, "session", &user); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to refresh session",
		})
	}
	return c.JSON(fiber.Map{"ok": true})
}

// ValidateSession returns the current user for a valid web session.
func ValidateSession(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "SESSION_EXPIRED"})
	}
	var user model.User
	if result := db.DB.First(&user, userID); result.Error != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "SESSION_EXPIRED"})
	}
	return c.JSON(fiber.Map{"user": SanitizedUser(&user)})
}

// AdminLogin authenticates an admin and issues admin session cookies.
func AdminLogin(c *fiber.Ctx) error {
	req := new(LoginRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}
	if req.Username == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "username and password are required",
		})
	}

	var user model.User
	if result := db.DB.Where("username = ?", req.Username).First(&user); result.Error != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid username or password",
		})
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid username or password",
		})
	}
	if user.Status != "active" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "account is not active",
		})
	}
	if user.Role != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "admin access required",
		})
	}

	if err := setAdminAuthCookies(c, &user); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to establish session",
		})
	}

	return c.JSON(fiber.Map{
		"user": SanitizedUser(&user),
		"role": user.Role,
	})
}
