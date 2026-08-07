package handler

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
)

func setupTestAppWithOrderRoutes(userID uint) *fiber.App {
	app := fiber.New()
	app.Post("/api/v1/user/orders", func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		return c.Next()
	}, CreateOrder)
	return app
}

func setupTestAppWithPaymentCallback() *fiber.App {
	app := fiber.New()
	app.Post("/api/v1/public/payment/callback", MockPayCallback)
	app.Post("/api/v1/public/payment/callback/:provider", PaymentCallback)
	return app
}

func createTestUser(t *testing.T) *model.User {
	t.Helper()
	user := model.User{
		Username:     "testuser",
		PasswordHash: "hash",
		Role:         "user",
		Status:       "active",
	}
	if result := db.DB.Create(&user); result.Error != nil {
		t.Fatalf("failed to create user: %v", result.Error)
	}
	return &user
}

func createTestProduct(t *testing.T, stock int) *model.Product {
	t.Helper()
	product := model.Product{
		Name:   "Test Product",
		Type:   "traffic",
		Price:  9.99,
		Stock:  stock,
		Status: "active",
	}
	if result := db.DB.Create(&product); result.Error != nil {
		t.Fatalf("failed to create product: %v", result.Error)
	}
	return &product
}

func TestCreateOrder_Success(t *testing.T) {
	setupTestDB(t)
	user := createTestUser(t)
	createTestProduct(t, 10)
	app := setupTestAppWithOrderRoutes(user.ID)

	body := `{"product_id":1}`
	req := httptest.NewRequest("POST", "/api/v1/user/orders", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}

	var result struct {
		Order model.Order `json:"order"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if result.Order.Status != "pending" {
		t.Fatalf("expected order status pending, got %s", result.Order.Status)
	}
}

func TestCreateOrder_NoStock(t *testing.T) {
	setupTestDB(t)
	user := createTestUser(t)
	createTestProduct(t, 0)
	app := setupTestAppWithOrderRoutes(user.ID)

	body := `{"product_id":1}`
	req := httptest.NewRequest("POST", "/api/v1/user/orders", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}
}

func TestCreateOrder_NonexistentProduct(t *testing.T) {
	setupTestDB(t)
	user := createTestUser(t)
	app := setupTestAppWithOrderRoutes(user.ID)

	body := `{"product_id":999}`
	req := httptest.NewRequest("POST", "/api/v1/user/orders", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected status 404, got %d", resp.StatusCode)
	}
}

func TestMockPayCallback_Paid(t *testing.T) {
	t.Setenv("MOCK_PAY_ENABLED", "1")
	setupTestDB(t)
	user := createTestUser(t)
	product := createTestProduct(t, 10)
	order := model.Order{
		UserID:    user.ID,
		ProductID: product.ID,
		Amount:    9.99,
		Status:    "pending",
	}
	if result := db.DB.Create(&order); result.Error != nil {
		t.Fatalf("failed to create order: %v", result.Error)
	}

	app := setupTestAppWithPaymentCallback()
	body := `{"order_id":1,"status":"paid"}`
	req := httptest.NewRequest("POST", "/api/v1/public/payment/callback", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var updatedOrder model.Order
	if result := db.DB.First(&updatedOrder, order.ID); result.Error != nil {
		t.Fatalf("failed to reload order: %v", result.Error)
	}
	if updatedOrder.Status != "paid" {
		t.Fatalf("expected order status paid, got %s", updatedOrder.Status)
	}

	// The mock callback must activate the subscription, matching the real
	// PaymentCallback path (stock is decremented at order creation instead).
	var updatedUser model.User
	if result := db.DB.First(&updatedUser, user.ID); result.Error != nil {
		t.Fatalf("failed to reload user: %v", result.Error)
	}
	if updatedUser.SubscriptionStatus != "active" {
		t.Fatalf("expected subscription active, got %s", updatedUser.SubscriptionStatus)
	}
	if updatedUser.ExpireTime <= 0 {
		t.Fatalf("expected expire_time set, got %d", updatedUser.ExpireTime)
	}

	var updatedProduct model.Product
	if result := db.DB.First(&updatedProduct, product.ID); result.Error != nil {
		t.Fatalf("failed to reload product: %v", result.Error)
	}
	if updatedProduct.Stock != 10 {
		t.Fatalf("expected product stock unchanged (10), got %d", updatedProduct.Stock)
	}
}

func TestMockPayCallback_AlreadyPaid(t *testing.T) {
	t.Setenv("MOCK_PAY_ENABLED", "1")
	setupTestDB(t)
	user := createTestUser(t)
	product := createTestProduct(t, 10)
	order := model.Order{
		UserID:    user.ID,
		ProductID: product.ID,
		Amount:    9.99,
		Status:    "paid",
	}
	if result := db.DB.Create(&order); result.Error != nil {
		t.Fatalf("failed to create order: %v", result.Error)
	}

	app := setupTestAppWithPaymentCallback()
	body := `{"order_id":1,"status":"paid"}`
	req := httptest.NewRequest("POST", "/api/v1/public/payment/callback", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}
}

func TestMockPayCallback_Cancelled(t *testing.T) {
	t.Setenv("MOCK_PAY_ENABLED", "1")
	setupTestDB(t)
	user := createTestUser(t)
	product := createTestProduct(t, 10)
	order := model.Order{
		UserID:    user.ID,
		ProductID: product.ID,
		Amount:    9.99,
		Status:    "pending",
	}
	if result := db.DB.Create(&order); result.Error != nil {
		t.Fatalf("failed to create order: %v", result.Error)
	}

	app := setupTestAppWithPaymentCallback()
	body := `{"order_id":1,"status":"cancelled"}`
	req := httptest.NewRequest("POST", "/api/v1/public/payment/callback", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var updatedOrder model.Order
	if result := db.DB.First(&updatedOrder, order.ID); result.Error != nil {
		t.Fatalf("failed to reload order: %v", result.Error)
	}
	if updatedOrder.Status != "cancelled" {
		t.Fatalf("expected order status cancelled, got %s", updatedOrder.Status)
	}
}

// The mock payment endpoints must fail closed (403) unless MOCK_PAY_ENABLED=1,
// so a default production deployment cannot be used to activate subscriptions for free.
func TestMockPayCallback_FailsClosedByDefault(t *testing.T) {
	setupTestDB(t)
	user := createTestUser(t)
	product := createTestProduct(t, 10)
	order := model.Order{
		UserID:    user.ID,
		ProductID: product.ID,
		Amount:    9.99,
		Status:    "pending",
		Provider:  "mock",
	}
	if result := db.DB.Create(&order); result.Error != nil {
		t.Fatalf("failed to create order: %v", result.Error)
	}

	app := setupTestAppWithPaymentCallback()

	// Direct mock callback route.
	req := httptest.NewRequest("POST", "/api/v1/public/payment/callback", bytes.NewReader([]byte(`{"order_id":1,"status":"paid"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403 for mock callback without MOCK_PAY_ENABLED, got %d", resp.StatusCode)
	}

	// Provider-routed mock callback.
	req2 := httptest.NewRequest("POST", "/api/v1/public/payment/callback/mock", bytes.NewReader([]byte(`{"order_id":1,"status":"paid"}`)))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp2.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403 for provider mock callback without MOCK_PAY_ENABLED, got %d", resp2.StatusCode)
	}

	// Order must remain pending.
	var updatedOrder model.Order
	if result := db.DB.First(&updatedOrder, order.ID); result.Error != nil {
		t.Fatalf("failed to reload order: %v", result.Error)
	}
	if updatedOrder.Status != "pending" {
		t.Fatalf("expected order to remain pending, got %s", updatedOrder.Status)
	}
}
