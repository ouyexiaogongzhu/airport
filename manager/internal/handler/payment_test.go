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

	var updatedProduct model.Product
	if result := db.DB.First(&updatedProduct, product.ID); result.Error != nil {
		t.Fatalf("failed to reload product: %v", result.Error)
	}
	if updatedProduct.Stock != 9 {
		t.Fatalf("expected product stock 9, got %d", updatedProduct.Stock)
	}
}

func TestMockPayCallback_AlreadyPaid(t *testing.T) {
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
