package handler

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
	"gorm.io/gorm"
)

const gbBytes = 1073741824
const subscriptionDays = 30 * 86400

type orderWithProduct struct {
	model.Order
	Product model.Product `gorm:"foreignKey:ProductID" json:"product"`
}

// CreateOrder creates a new order for the authenticated user.
func CreateOrder(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	req := new(model.CreateOrderInput)
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

	providerName := req.Provider
	if providerName == "" {
		providerName = "mock"
	}

	if _, ok := GetPaymentProvider(providerName); !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid payment provider",
		})
	}

	order := model.Order{
		UserID:    userID,
		ProductID: req.ProductID,
		Amount:    product.Price,
		Status:    "pending",
		Provider:  providerName,
	}
	if result := db.DB.Create(&order); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create order",
		})
	}

	if providerName != "mock" {
		provider, _ := GetPaymentProvider(providerName)
		notifyURL := fmt.Sprintf("%s://%s/api/v1/payment/callback/%s", c.Protocol(), c.Hostname(), providerName)
		returnURL := fmt.Sprintf("%s://%s/user/orders/%d", c.Protocol(), c.Hostname(), order.ID)

		paymentURL, err := provider.CreatePayment(&PaymentOrder{
			ID:        order.ID,
			UserID:    userID,
			Amount:    order.Amount,
			NotifyURL: notifyURL,
			ReturnURL: returnURL,
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to create payment",
			})
		}
		order.PaymentURL = paymentURL
		if result := db.DB.Model(&order).Update("payment_url", paymentURL); result.Error != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to save payment url",
			})
		}
	}

	if result := db.DB.Model(&model.Product{}).Where("id = ?", req.ProductID).
		Update("stock", gorm.Expr("stock - 1")); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to update product stock",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"order":       order,
		"payment_url": order.PaymentURL,
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

// GetOrder returns a single order for the authenticated user.
func GetOrder(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid order id",
		})
	}

	var order model.Order
	if result := db.DB.First(&order, id); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "order not found",
		})
	}

	if order.UserID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "forbidden",
		})
	}

	return c.JSON(order)
}

// PaymentCallback handles payment provider webhook callbacks.
func PaymentCallback(c *fiber.Ctx) error {
	providerName := c.Params("provider")
	provider, ok := GetPaymentProvider(providerName)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid payment provider",
		})
	}

	result, err := provider.VerifyCallback(c)
	if err != nil {
		if e, ok := err.(*fiber.Error); ok {
			return c.Status(e.Code).JSON(fiber.Map{"error": e.Message})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	orderID, err := strconv.ParseUint(result.OrderID, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid order id in callback",
		})
	}

	var order model.Order
	if dbResult := db.DB.First(&order, orderID); dbResult.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "order not found",
		})
	}

	if result.Status != "paid" {
		db.DB.Model(&order).Update("status", "failed")
		return c.SendStatus(fiber.StatusOK)
	}

	if order.Status == "paid" {
		return c.SendString("ok")
	}

	if order.Status != "pending" {
		return c.SendString("ok")
	}

	if err := db.DB.Model(&order).Update("status", "paid").Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to update order",
		})
	}

	var user model.User
	if dbResult := db.DB.First(&user, order.UserID); dbResult.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "user not found",
		})
	}

	var product model.Product
	if dbResult := db.DB.First(&product, order.ProductID); dbResult.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "product not found",
		})
	}

	if err := activateSubscription(&user, &product, order.Amount); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to activate subscription",
		})
	}

	return c.SendString("ok")
}

func activateSubscription(user *model.User, product *model.Product, amount float64) error {
	now := time.Now().Unix()
	trafficLimit := int64(amount * gbBytes)

	updates := map[string]interface{}{
		"subscription_status":  "active",
		"subscription_tier":    product.Name,
		"traffic_limit_bytes":  trafficLimit,
		"rate_limit_bps":       int64(0),
		"traffic_used_bytes":   int64(0),
		"traffic_period_start": now,
	}

	if user.SubscriptionStatus == "active" && user.ExpireTime > 0 {
		updates["expire_time"] = user.ExpireTime + subscriptionDays
	} else {
		updates["expire_time"] = now + subscriptionDays
	}

	return db.DB.Model(user).Updates(updates).Error
}

// AdminListOrders returns all orders with product details.
func AdminListOrders(c *fiber.Ctx) error {
	var orders []orderWithProduct
	if result := db.DB.Preload("Product").Order("created_at DESC").Find(&orders); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to list orders",
		})
	}

	return c.JSON(orders)
}

// AdminGetOrder returns a single order with product details.
func AdminGetOrder(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid order id",
		})
	}

	var order orderWithProduct
	if result := db.DB.Preload("Product").First(&order, id); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "order not found",
		})
	}

	return c.JSON(order)
}

// AdminRefundOrder refunds an order and restores product stock.
func AdminRefundOrder(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid order id",
		})
	}

	var order model.Order
	if result := db.DB.First(&order, id); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "order not found",
		})
	}

	if order.Status == "refunded" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "order already refunded",
		})
	}

	if err := db.DB.Model(&order).Update("status", "refunded").Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to refund order",
		})
	}

	if result := db.DB.Model(&model.Product{}).Where("id = ?", order.ProductID).
		Update("stock", gorm.Expr("stock + 1")); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to restore product stock",
		})
	}

	return c.JSON(fiber.Map{
		"message": "order refunded",
		"order":   order,
	})
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
		db.DB.Model(&order).Update("status", "paid")

		db.DB.Model(&model.Product{}).Where("id = ?", order.ProductID).
			Update("stock", gorm.Expr("stock - 1"))

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
