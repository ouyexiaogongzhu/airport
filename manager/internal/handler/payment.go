package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
	"gorm.io/gorm"
)

type CreateOrderRequest struct {
	ProductID uint `json:"product_id" validate:"required"`
}

// CreateOrder creates a new order for the authenticated user.
func CreateOrder(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	req := new(CreateOrderRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.ProductID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "product_id is required",
		})
	}

	// Find the product
	var product model.Product
	if result := db.DB.First(&product, req.ProductID); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "product not found",
		})
	}

	if product.Status != "active" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "product is not available",
		})
	}

	if product.Stock <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "product out of stock",
		})
	}

	// Create order
	order := model.Order{
		UserID:    userID,
		ProductID: req.ProductID,
		Amount:    product.Price,
		Status:    "pending",
	}
	if result := db.DB.Create(&order); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create order",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"order":          order,
		"payment_method": "mock",
		"mock_pay_url":   "/api/v1/public/payment/callback",
	})
}

// ListOrders returns all orders for the authenticated user.
func ListOrders(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	var orders []model.Order
	if result := db.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&orders); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to list orders",
		})
	}

	return c.JSON(orders)
}

type MockPayCallbackRequest struct {
	OrderID uint   `json:"order_id" validate:"required"`
	Status  string `json:"status" validate:"required"` // paid, cancelled
}

// MockPayCallback simulates a payment gateway callback.
func MockPayCallback(c *fiber.Ctx) error {
	req := new(MockPayCallbackRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.OrderID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "order_id is required",
		})
	}

	// Find the order
	var order model.Order
	if result := db.DB.First(&order, req.OrderID); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "order not found",
		})
	}

	if order.Status != "pending" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "order is not in pending status",
		})
	}

	switch req.Status {
	case "paid":
		// Update order status
		db.DB.Model(&order).Update("status", "paid")

		// Decrease product stock
		db.DB.Model(&model.Product{}).Where("id = ?", order.ProductID).
			Update("stock", gorm.Expr("stock - 1"))

		// Add balance to user (mock: credit balance equal to order amount)
		db.DB.Model(&model.User{}).Where("id = ?", order.UserID).
			Update("balance", gorm.Expr("balance + ?", order.Amount))

		return c.JSON(fiber.Map{
			"message": "payment successful",
			"order":   order,
		})

	case "cancelled":
		db.DB.Model(&order).Update("status", "cancelled")
		return c.JSON(fiber.Map{
			"message": "payment cancelled",
			"order":   order,
		})

	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid status, must be 'paid' or 'cancelled'",
		})
	}
}
