package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

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

// bepusdtSign computes the BEpusdt request signature: all non-empty params
// (excluding signature) sorted by ASCII key, joined as k=v&k=v, with the API
// token appended and the result MD5-hashed (lowercase).
func bepusdtSign(params map[string]string, token string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "signature" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(params[k])
	}
	b.WriteString(token)

	sum := md5.Sum([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// CreatePayment creates a BEpusdt order via the provider's REST API and
// returns the hosted payment page URL.
func (p *BEpusdtProvider) CreatePayment(order *PaymentOrder) (string, error) {
	host := strings.TrimRight(p.Host, "/")
	if host == "" {
		host = strings.TrimRight(os.Getenv("BEPUSDT_API_URL"), "/")
	}
	token := p.AuthToken
	if token == "" {
		token = os.Getenv("BEPUSDT_TOKEN")
	}
	if host == "" || token == "" {
		return "", fmt.Errorf("bepusdt: BEPUSDT_API_URL and BEPUSDT_TOKEN are required")
	}

	orderID := strconv.FormatUint(uint64(order.ID), 10)
	amount := strconv.FormatFloat(order.Amount, 'f', -1, 64)
	params := map[string]string{
		"order_id":     orderID,
		"amount":       amount,
		"notify_url":   order.NotifyURL,
		"redirect_url": order.ReturnURL,
	}
	params["signature"] = bepusdtSign(params, token)

	payload, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("bepusdt: marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, host+"/api/v1/order/create-transaction", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("bepusdt: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("bepusdt: request: %w", err)
	}
	defer resp.Body.Close()

	var apiResp struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
		Data    struct {
			TradeID string `json:"trade_id"`
			URL     string `json:"url"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("bepusdt: decode response: %w", err)
	}
	if !apiResp.Status || apiResp.Data.URL == "" {
		return "", fmt.Errorf("bepusdt: order rejected: %s", apiResp.Message)
	}
	return apiResp.Data.URL, nil
}

func (p *BEpusdtProvider) VerifyCallback(c *fiber.Ctx) (*CallbackResult, error) {
	var body struct {
		OrderID            string `json:"order_id"`
		Amount             string `json:"amount"`
		ActualAmount       string `json:"actual_amount"`
		Token              string `json:"token"`
		Status             int    `json:"status"`
		BlockTransactionID string `json:"block_transaction_id"`
		CreatedAt          string `json:"created_at"`
		ExpiredAt          string `json:"expired_at"`
		Signature          string `json:"signature"`
	}
	if err := c.BodyParser(&body); err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid callback body")
	}

	// Verify the callback signature using the same BEpusdt signing rule.
	// Fails closed: if no secret is configured the callback is rejected outright
	// rather than being trusted unsigned.
	secret := os.Getenv("BEPUSDT_SECRET")
	if secret == "" {
		secret = os.Getenv("BEPUSDT_TOKEN")
	}
	if secret == "" {
		return nil, fiber.NewError(fiber.StatusServiceUnavailable, "BEPUSDT_SECRET not configured")
	}
	params := map[string]string{
		"order_id":             body.OrderID,
		"amount":               body.Amount,
		"actual_amount":        body.ActualAmount,
		"token":                body.Token,
		"status":               strconv.Itoa(body.Status),
		"block_transaction_id": body.BlockTransactionID,
		"created_at":           body.CreatedAt,
		"expired_at":           body.ExpiredAt,
	}
	expected := bepusdtSign(params, secret)
	if body.Signature == "" || !hmac.Equal([]byte(strings.ToLower(body.Signature)), []byte(expected)) {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid signature")
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

	// Verify HMAC signature. Fails closed: unconfigured webhook secret rejects
	// the callback outright instead of trusting an unsigned request.
	secret := os.Getenv("PAYONEER_WEBHOOK_SECRET")
	if secret == "" {
		return nil, fiber.NewError(fiber.StatusServiceUnavailable, "PAYONEER_WEBHOOK_SECRET not configured")
	}
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
	RegisterPaymentProvider(&BEpusdtProvider{
		Host:      os.Getenv("BEPUSDT_API_URL"),
		AuthToken: os.Getenv("BEPUSDT_TOKEN"),
	})
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
