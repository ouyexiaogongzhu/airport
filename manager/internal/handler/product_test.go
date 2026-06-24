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

func setupTestAppWithProductRoutes() *fiber.App {
	app := fiber.New()
	admin := app.Group("/api/v1/admin")
	admin.Post("/products", CreateProduct)
	admin.Get("/products", ListProducts)
	admin.Put("/products/:id", UpdateProduct)
	admin.Delete("/products/:id", DeleteProduct)

	api := app.Group("/api/v1")
	api.Get("/products", ListActiveProducts)
	return app
}

func TestListProducts_Empty(t *testing.T) {
	setupTestDB(t)
	app := setupTestAppWithProductRoutes()

	req := httptest.NewRequest("GET", "/api/v1/admin/products", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var result struct {
		Products []model.Product `json:"products"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(result.Products) != 0 {
		t.Fatalf("expected empty list, got %d items", len(result.Products))
	}
}

func TestCreateProduct_Success(t *testing.T) {
	setupTestDB(t)
	app := setupTestAppWithProductRoutes()

	body := `{"name":"Test Plan","type":"monthly","price":19.99,"stock":100}`
	req := httptest.NewRequest("POST", "/api/v1/admin/products", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}

	var result struct {
		Product model.Product `json:"product"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if result.Product.Name != "Test Plan" {
		t.Fatalf("expected name 'Test Plan', got %s", result.Product.Name)
	}
	if result.Product.Price != 19.99 {
		t.Fatalf("expected price 19.99, got %f", result.Product.Price)
	}
	if result.Product.Status != "active" {
		t.Fatalf("expected status 'active', got %s", result.Product.Status)
	}
}

func TestCreateProduct_ValidationError(t *testing.T) {
	setupTestDB(t)
	app := setupTestAppWithProductRoutes()

	// Missing name and invalid price
	body := `{"price":-1}`
	req := httptest.NewRequest("POST", "/api/v1/admin/products", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected status 400 for validation error, got %d", resp.StatusCode)
	}
}

func TestListProducts_AfterCreate(t *testing.T) {
	setupTestDB(t)
	app := setupTestAppWithProductRoutes()

	// Create a product
	body := `{"name":"Premium Plan","type":"yearly","price":99.99,"stock":50}`
	req := httptest.NewRequest("POST", "/api/v1/admin/products", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201 on create, got %d", resp.StatusCode)
	}

	// List products and verify
	req = httptest.NewRequest("GET", "/api/v1/admin/products", nil)
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 on list, got %d", resp.StatusCode)
	}

	var result struct {
		Products []model.Product `json:"products"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(result.Products) != 1 {
		t.Fatalf("expected 1 product, got %d", len(result.Products))
	}
	if result.Products[0].Name != "Premium Plan" {
		t.Fatalf("expected product name 'Premium Plan', got %s", result.Products[0].Name)
	}
}

func TestUpdateProduct(t *testing.T) {
	setupTestDB(t)
	app := setupTestAppWithProductRoutes()

	// Create a product first
	body := `{"name":"Original","type":"monthly","price":9.99,"stock":10}`
	req := httptest.NewRequest("POST", "/api/v1/admin/products", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201 on create, got %d", resp.StatusCode)
	}

	// Update the product
	updateBody := `{"name":"Updated Name","price":29.99}`
	req = httptest.NewRequest("PUT", "/api/v1/admin/products/1", bytes.NewReader([]byte(updateBody)))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("update request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 on update, got %d", resp.StatusCode)
	}

	var result struct {
		Product model.Product `json:"product"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to parse update response: %v", err)
	}
	if result.Product.Name != "Updated Name" {
		t.Fatalf("expected name 'Updated Name', got %s", result.Product.Name)
	}
	if result.Product.Price != 29.99 {
		t.Fatalf("expected price 29.99, got %f", result.Product.Price)
	}
	if result.Product.Stock != 10 {
		t.Fatalf("expected stock unchanged at 10, got %d", result.Product.Stock)
	}
}

func TestPublicListActiveProducts(t *testing.T) {
	setupTestDB(t)
	app := setupTestAppWithProductRoutes()

	// Create one active product
	body := `{"name":"Active Plan","type":"monthly","price":15.00,"stock":99}`
	req := httptest.NewRequest("POST", "/api/v1/admin/products", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201 on create, got %d", resp.StatusCode)
	}

	// Public endpoint should return the active product
	req = httptest.NewRequest("GET", "/api/v1/products", nil)
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("public list request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 on public list, got %d", resp.StatusCode)
	}

	var result struct {
		Products []model.Product `json:"products"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to parse public response: %v", err)
	}
	if len(result.Products) != 1 {
		t.Fatalf("expected 1 active product, got %d", len(result.Products))
	}
	if result.Products[0].Name != "Active Plan" {
		t.Fatalf("expected name 'Active Plan', got %s", result.Products[0].Name)
	}
}

func TestArchiveProduct(t *testing.T) {
	setupTestDB(t)
	app := setupTestAppWithProductRoutes()

	// Create a product
	body := `{"name":"To Archive","type":"monthly","price":5.00,"stock":5}`
	req := httptest.NewRequest("POST", "/api/v1/admin/products", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201 on create, got %d", resp.StatusCode)
	}

	// Archive (delete) the product
	req = httptest.NewRequest("DELETE", "/api/v1/admin/products/1", nil)
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("delete request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 on archive, got %d", resp.StatusCode)
	}

	// Verify product is still in DB but with status=archived
	var product model.Product
	if result := db.DB.First(&product, 1); result.Error != nil {
		t.Fatalf("product should still exist: %v", result.Error)
	}
	if product.Status != "archived" {
		t.Fatalf("expected status 'archived', got %s", product.Status)
	}

	// Public endpoint should NOT return archived product
	req = httptest.NewRequest("GET", "/api/v1/products", nil)
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("public list request failed: %v", err)
	}

	var result struct {
		Products []model.Product `json:"products"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to parse public response: %v", err)
	}
	if len(result.Products) != 0 {
		t.Fatalf("expected 0 active products after archive, got %d", len(result.Products))
	}
}

func TestDeleteProduct_NotFound(t *testing.T) {
	setupTestDB(t)
	app := setupTestAppWithProductRoutes()

	req := httptest.NewRequest("DELETE", "/api/v1/admin/products/999", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected 404 for non-existent product, got %d", resp.StatusCode)
	}
}
