package handler

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/gofiber/fiber/v2"
	stripe "github.com/stripe/stripe-go/v81"
	stripeSession "github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/webhook"
)

const (
	envStripeSecret        = "STRIPE_SECRET"
	envStripeKey           = "STRIPE_KEY"
	envStripeWebhookSecret = "STRIPE_WEBHOOK_SECRET"
)

// StripeProvider implements PaymentProvider for Stripe Checkout.
type StripeProvider struct{}

func (p *StripeProvider) Name() string { return "stripe" }

// CreatePayment creates a Stripe Checkout Session and returns its URL.
func (p *StripeProvider) CreatePayment(order *PaymentOrder) (string, error) {
	secret := os.Getenv(envStripeSecret)
	if secret == "" {
		return "", fmt.Errorf("stripe: %s not set", envStripeSecret)
	}
	stripe.Key = secret

	currency := "cny"
	unitAmount := int64(order.Amount * 100) // Stripe uses cents/smallest unit

	params := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: &currency,
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(fmt.Sprintf("Order #%d", order.ID)),
					},
					UnitAmount: &unitAmount,
				},
				Quantity: stripe.Int64(1),
			},
		},
		ClientReferenceID: stripe.String(fmt.Sprintf("%d", order.ID)),
		SuccessURL:        stripe.String(order.ReturnURL),
		CancelURL:         stripe.String(order.ReturnURL),
	}

	s, err := stripeSession.New(params)
	if err != nil {
		return "", fmt.Errorf("stripe: create session: %w", err)
	}
	if s.URL == "" {
		return "", fmt.Errorf("stripe: session created but URL is empty")
	}
	return s.URL, nil
}

// VerifyCallback verifies Stripe webhook signature and returns the callback result.
func (p *StripeProvider) VerifyCallback(c *fiber.Ctx) (*CallbackResult, error) {
	secret := os.Getenv(envStripeSecret)
	if secret == "" {
		return nil, fiber.NewError(fiber.StatusInternalServerError,
			"stripe: "+envStripeSecret+" not set")
	}
	stripe.Key = secret

	webhookSecret := os.Getenv(envStripeWebhookSecret)
	if webhookSecret == "" {
		return nil, fiber.NewError(fiber.StatusInternalServerError,
			"stripe: "+envStripeWebhookSecret+" not set")
	}

	payload := c.Body()
	sigHeader := c.Get("Stripe-Signature")
	if sigHeader == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest,
			"stripe: missing Stripe-Signature header")
	}

	event, err := webhook.ConstructEvent(payload, sigHeader, webhookSecret)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest,
			"stripe: invalid webhook signature: "+err.Error())
	}

	status := "failed"
	var orderID string

	switch event.Type {
	case stripe.EventTypeCheckoutSessionCompleted:
		var session stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			return nil, fiber.NewError(fiber.StatusBadRequest,
				"stripe: failed to parse session: "+err.Error())
		}
		if session.PaymentStatus == stripe.CheckoutSessionPaymentStatusPaid {
			status = "paid"
		}
		orderID = session.ClientReferenceID

	case stripe.EventTypeCheckoutSessionAsyncPaymentFailed:
		var session stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			return nil, fiber.NewError(fiber.StatusBadRequest,
				"stripe: failed to parse session: "+err.Error())
		}
		orderID = session.ClientReferenceID

	case stripe.EventTypeCheckoutSessionExpired:
		var session stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			return nil, fiber.NewError(fiber.StatusBadRequest,
				"stripe: failed to parse session: "+err.Error())
		}
		orderID = session.ClientReferenceID

	default:
		// Acknowledge the webhook but don't process — unknown/uninteresting event
		return &CallbackResult{
			OrderID:         "",
			TransactionID:   event.ID,
			Status:          "ignored",
			ProviderOrderID: event.ID,
		}, nil
	}

	if orderID == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest,
			"stripe: unable to determine order ID from event")
	}

	return &CallbackResult{
		OrderID:         orderID,
		TransactionID:   fmt.Sprintf("stripe_tx_%s", event.ID),
		Status:          status,
		ProviderOrderID: event.ID,
	}, nil
}
