package handler

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
	"golang.org/x/crypto/bcrypt"
)

var registerLimits sync.Map

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
	Password string `json:"password" validate:"required,min=6"`
}

type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type TokenLoginRequest struct {
	Token string `json:"token"`
}

type AuthResponse struct {
	Token    string      `json:"token"`
	User     *model.User `json:"user"`
}

func getJWTSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "dev-secret"
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

	if len(req.Password) < 6 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "password must be at least 6 characters",
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

	if result := db.DB.Create(&user); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create user",
		})
	}

	// Generate client_token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		// non-fatal, continue
	}
	user.ClientToken = "rf_" + hex.EncodeToString(tokenBytes)
	db.DB.Model(&user).Update("client_token", user.ClientToken)

	// Generate JWT
	token, err := generateToken(&user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate token",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(AuthResponse{
		Token: token,
		User:  &user,
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

	token, err := generateToken(&user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate token",
		})
	}

	return c.JSON(AuthResponse{
		Token: token,
		User:  &user,
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
		User:  &user,
	})
}

func generateToken(user *model.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     user.Role,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(getJWTSecret())
}
