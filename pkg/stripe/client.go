package stripe

import (
	"context"
	"fmt"

	stripeSDK "github.com/stripe/stripe-go/v82"
	portalSession "github.com/stripe/stripe-go/v82/billingportal/session"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/webhook"
)

// Client wraps the Stripe SDK for billing operations.
type Client struct {
	secretKey string
}

// NewClient creates a new Stripe client with the given secret key.
func NewClient(secretKey string) *Client {
	stripeSDK.Key = secretKey
	return &Client{secretKey: secretKey}
}

// CreateCustomer creates a new Stripe customer and returns the customer ID.
func (c *Client) CreateCustomer(ctx context.Context, userID, username string) (string, error) {
	params := &stripeSDK.CustomerParams{
		Params: stripeSDK.Params{
			Metadata: map[string]string{
				"user_id": userID,
			},
		},
	}
	if username != "" {
		params.Name = stripeSDK.String(username)
	}

	cust, err := customer.New(params)
	if err != nil {
		return "", fmt.Errorf("stripe create customer: %w", err)
	}

	return string(cust.ID), nil
}

// CreateCheckoutSession creates a Stripe Checkout Session for subscription purchase.
func (c *Client) CreateCheckoutSession(ctx context.Context, customerID, priceID, frontendURL string) (string, string, error) {
	params := &stripeSDK.CheckoutSessionParams{
		Customer: stripeSDK.String(customerID),
		Mode:     stripeSDK.String(string(stripeSDK.CheckoutSessionModeSubscription)),
		LineItems: []*stripeSDK.CheckoutSessionLineItemParams{
			{
				Price:    stripeSDK.String(priceID),
				Quantity: stripeSDK.Int64(1),
			},
		},
		SuccessURL: stripeSDK.String(frontendURL + "/pricing?checkout=success"),
		CancelURL:  stripeSDK.String(frontendURL + "/pricing?checkout=canceled"),
	}

	s, err := session.New(params)
	if err != nil {
		return "", "", fmt.Errorf("stripe create checkout session: %w", err)
	}

	return s.URL, string(s.ID), nil
}

// CreateCustomerPortalSession creates a Stripe Customer Portal session.
func (c *Client) CreateCustomerPortalSession(ctx context.Context, customerID, frontendURL string) (string, error) {
	params := &stripeSDK.BillingPortalSessionParams{
		Customer:  stripeSDK.String(customerID),
		ReturnURL: stripeSDK.String(frontendURL + "/settings"),
	}

	s, err := portalSession.New(params)
	if err != nil {
		return "", fmt.Errorf("stripe create portal session: %w", err)
	}

	return s.URL, nil
}

// VerifyWebhookSignature verifies a Stripe webhook signature and returns the event type.
// The caller should use the original payload as the verified event data.
func (c *Client) VerifyWebhookSignature(payload []byte, signature string, webhookSecret string) (string, error) {
	event, err := webhook.ConstructEvent(payload, signature, webhookSecret)
	if err != nil {
		return "", fmt.Errorf("stripe webhook verification failed: %w", err)
	}

	return string(event.Type), nil
}
