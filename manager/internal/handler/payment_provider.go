package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/gofiber/fiber/v2"
)

// Payload returned after verifying callback
type CallbackResult struct {
	OrderID         string // external provider's order ID
	TransactionID   string // block tx / payoneer transaction id
	Status          string // "paid" or "failed"
	ProviderOrderID string // provider's own order reference
}

type PaymentProvider interface {
	Name() string
	CreatePayment(order *PaymentOrder) (paymentURL string, err error)
	VerifyCallback(ctx *fiber.Ctx) (*CallbackResult, error)
}

// PaymentOrder is the internal order representation passed to providers
type PaymentOrder struct {
	ID                uint
	UserID            uint
	Amount            float64
	NotifyURL         string
	ReturnURL         string
	ProviderOrderID   string
}

type MockProvider struct{}

func (p *MockProvider) Name() string { return "mock" }

func (p *MockProvider) CreatePayment(order *PaymentOrder) (string, error) {
	return "/api/v1/public/payment/callback", nil
}

func (p *MockProvider) VerifyCallback(c *fiber.Ctx) (*CallbackResult, error) {
	var req struct {
		OrderID uint   `json:"order_id"`
		Status  string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid callback body")
	}
	return &CallbackResult{
		OrderID:         fmt.Sprintf("%d", req.OrderID),
		TransactionID:   fmt.Sprintf("mock_tx_%d", req.OrderID),
		Status:          req.Status,
		ProviderOrderID: fmt.Sprintf("%d", req.OrderID),
	}, nil
}

type BEpusdtProvider struct {
	Host      string
	AuthToken string
}

func (p *BEpusdtProvider) Name() string { return "bepusdt" }

func (p *BEpusdtProvider) CreatePayment(order *PaymentOrder) (string, error) {
	return fmt.Sprintf("https://pay.bepusdt.example/pay?order_id=%d&amount=%.2f", order.ID, order.Amount), nil
}

func (p *BEpusdtProvider) VerifyCallback(c *fiber.Ctx) (*CallbackResult, error) {
	var body struct {
		OrderID            string `json:"order_id"`
		Amount             string `json:"amount"`
		ActualAmount       string `json:"actual_amount"`
		Token              string `json:"token"`
		Status             int    `json:"status"`
		BlockTransactionID string `json:"block_transaction_id"`
		Signature          string `json:"signature"`
	}
	if err := c.BodyParser(&body); err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid callback body")
	}

	// Verify HMAC signature if secret is configured
	secret := os.Getenv("BEPUSDT_SECRET")
	if secret != "" {
		if body.Signature == "" {
			return nil, fiber.NewError(fiber.StatusBadRequest, "missing signature")
		}
		payload := fmt.Sprintf("%s%s%s%s", body.OrderID, body.Amount, body.ActualAmount, body.Token)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(payload))
		expected := hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(body.Signature), []byte(expected)) {
			return nil, fiber.NewError(fiber.StatusBadRequest, "invalid signature")
		}
	}

	status := "failed"
	if body.Status == 2 {
		status = "paid"
	} else if body.Status == 1 {
		status = "pending"
	}

	return &CallbackResult{
		OrderID:         body.OrderID,
		TransactionID:   body.BlockTransactionID,
		Status:          status,
		ProviderOrderID: body.OrderID,
	}, nil
}

type PayoneerProvider struct {
	APIKey        string
	MerchantID    string
	WebhookSecret string
}

func (p *PayoneerProvider) Name() string { return "payoneer" }

func (p *PayoneerProvider) CreatePayment(order *PaymentOrder) (string, error) {
	return fmt.Sprintf("https://checkout.payoneer.example/session?order_id=%d", order.ID), nil
}

func (p *PayoneerProvider) VerifyCallback(c *fiber.Ctx) (*CallbackResult, error) {
	var body struct {
		ReferenceID   string `json:"reference_id"`
		EventType     string `json:"event_type"`
		TransactionID string `json:"transaction_id"`
		Status        string `json:"status"`
		Signature     string `json:"signature"`
	}
	if err := c.BodyParser(&body); err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid callback body")
	}

	// Verify HMAC signature if secret is configured
	secret := os.Getenv("PAYONEER_WEBHOOK_SECRET")
	if secret != "" {
		if body.Signature == "" {
			return nil, fiber.NewError(fiber.StatusBadRequest, "missing signature")
		}
		payload := fmt.Sprintf("%s%s%s%s", body.ReferenceID, body.EventType, body.TransactionID, body.Status)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(payload))
		expected := hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(body.Signature), []byte(expected)) {
			return nil, fiber.NewError(fiber.StatusBadRequest, "invalid signature")
		}
	}

	status := "failed"
	if body.EventType == "payment.completed" || body.Status == "completed" {
		status = "paid"
	}

	return &CallbackResult{
		OrderID:         body.ReferenceID,
		TransactionID:   body.TransactionID,
		Status:          status,
		ProviderOrderID: body.ReferenceID,
	}, nil
}

var paymentProviders map[string]PaymentProvider

func init() {
	paymentProviders = make(map[string]PaymentProvider)
	RegisterPaymentProvider(&MockProvider{})
	RegisterPaymentProvider(&BEpusdtProvider{})
	RegisterPaymentProvider(&PayoneerProvider{})
	RegisterPaymentProvider(&StripeProvider{})
}

func RegisterPaymentProvider(p PaymentProvider) {
	paymentProviders[p.Name()] = p
}

func GetPaymentProvider(name string) (PaymentProvider, bool) {
	p, ok := paymentProviders[name]
	return p, ok
}
