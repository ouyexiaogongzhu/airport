package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
	"gorm.io/gorm"
)

const gbBytes = 1073741824
const subscriptionDurationSeconds = 30 * 86400

type orderWithProduct struct {
	model.Order
	Product model.Product `gorm:"foreignKey:ProductID" json:"product"`
}

// CreateOrder creates a new order for the authenticated user.
// TODO: Propagate request context via c.Context() for GORM queries.
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

	providerName := req.Provider
	if providerName == "" {
		providerName = "mock"
	}

	provider, ok := GetPaymentProvider(providerName)
	if !ok {
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

	// Create order and decrement stock atomically
	if err := db.DB.Transaction(func(tx *gorm.DB) error {
		// Lock the product row and check stock
		var lockedProduct model.Product
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&lockedProduct, req.ProductID).Error; err != nil {
			return err
		}
		if lockedProduct.Stock <= 0 {
			return fmt.Errorf("out of stock")
		}
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		return tx.Model(&model.Product{}).Where("id = ? AND stock > 0", req.ProductID).
			Update("stock", gorm.Expr("stock - 1")).Error
	}); err != nil {
		if err.Error() == "out of stock" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "product out of stock",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create order",
		})
	}

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

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"order":       order,
		"payment_url": order.PaymentURL,
	})
}

// ListOrders returns all orders for the authenticated user with pagination.
func ListOrders(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	offset, limit := parsePagination(c)

	var total int64
	db.DB.Model(&model.Order{}).Where("user_id = ?", userID).Count(&total)

	var orders []model.Order
	if result := db.DB.Where("user_id = ?", userID).Order("created_at DESC").Offset(offset).Limit(limit).Find(&orders); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to list orders",
		})
	}

	return c.JSON(fiber.Map{
		"data":     orders,
		"total":    total,
		"page":     offset/limit + 1,
		"per_page": limit,
	})
}

// GetOrder returns a single order for the authenticated user.
func GetOrder(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
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

	if order.Status == "paid" {
		return c.SendString("ok")
	}

	if result.Status == "paid" && order.Status == "pending" {
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

		if err := db.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&order).Update("status", "paid").Error; err != nil {
				return err
			}
			return activateSubscriptionTx(tx, &user, &product, order.Amount)
		}); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to activate subscription",
			})
		}

		return c.SendString("ok")
	}

	if result.Status == "failed" || result.Status == "expired" {
		if err := db.DB.Model(&order).Update("status", result.Status).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to update order status",
			})
		}
	}

	return c.SendString("ok")
}

func activateSubscription(user *model.User, product *model.Product, amount float64) error {
	return activateSubscriptionTx(db.DB, user, product, amount)
}

func activateSubscriptionTx(tx *gorm.DB, user *model.User, product *model.Product, amount float64) error {
	now := time.Now().Unix()
	trafficLimit := int64(amount * gbBytes)

	if user.SubscriptionStatus == "active" {
		return tx.Model(user).Update("expire_time", user.ExpireTime+subscriptionDurationSeconds).Error
	}

	updates := map[string]interface{}{
		"subscription_status":  "active",
		"subscription_tier":    product.Name,
		"traffic_limit_bytes":  trafficLimit,
		"rate_limit_bps":       int64(0),
		"traffic_used_bytes":   int64(0),
		"traffic_period_start": now,
		"expire_time":          now + subscriptionDurationSeconds,
	}

	return tx.Model(user).Updates(updates).Error
}

// AdminListOrders returns all orders with product details and pagination.
func AdminListOrders(c *fiber.Ctx) error {
	offset, limit := parsePagination(c)

	var total int64
	db.DB.Model(&model.Order{}).Count(&total)

	var orders []orderWithProduct
	if result := db.DB.Preload("Product").Order("created_at DESC").Offset(offset).Limit(limit).Find(&orders); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to list orders",
		})
	}

	return c.JSON(fiber.Map{
		"data":     orders,
		"total":    total,
		"page":     offset/limit + 1,
		"per_page": limit,
	})
}

// AdminGetOrder returns a single order with product details.
func AdminGetOrder(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
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
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
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

	if order.Status != "paid" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "can only refund paid orders",
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

// MockPayCallback simulates a payment gateway callback. Only available in dev/test.
func MockPayCallback(c *fiber.Ctx) error {
	if os.Getenv("MOCK_PAY_ENABLED") == "" && os.Getenv("APP_ENV") == "production" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "mock payment callback is disabled in production",
		})
	}
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
		if err := db.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&order).Update("status", "paid").Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Product{}).Where("id = ? AND stock > 0", order.ProductID).
				Update("stock", gorm.Expr("stock - 1")).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.User{}).Where("id = ?", order.UserID).
				Update("balance", gorm.Expr("balance + ?", order.Amount)).Error; err != nil {
				return err
			}
			return nil
		}); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to process payment",
			})
		}

		return c.JSON(fiber.Map{
			"message": "payment successful",
			"order":   order,
		})

	case "cancelled":
		if err := db.DB.Model(&order).Update("status", "cancelled").Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to cancel order",
			})
		}
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
