// Package paddle implements a thin client for the Paddle Billing API
// (https://developer.paddle.com/api-reference) plus verification and parsing
// of Paddle webhook notifications. Paddle is the merchant of record for web
// checkout; mobile subscriptions stay on RevenueCat (see pkg/revenuecat).
//
// The package covers the small slice of the API the backend needs:
//
//   - Creating checkout transactions (POST /transactions) and returning the
//     payment link (transaction.checkout.url) that the frontend opens with
//     Paddle.js. Transactions reference customers by Paddle ID (ctm_...), so
//     when only an email is known a customer is created first
//     (POST /customers).
//   - Reading subscriptions (GET /subscriptions/{id},
//     GET /subscriptions?customer_id=...) and canceling them at the end of the
//     current billing period (POST /subscriptions/{id}/cancel with
//     effective_from=next_billing_period).
//   - Verifying the Paddle-Signature header on incoming webhooks and parsing
//     the event envelope.
//
// # Webhooks
//
// A webhook handler should verify the raw request body before parsing it —
// verification is an HMAC-SHA256 over "ts:body" so the body must be the exact
// bytes received, not re-serialized JSON:
//
//	if err := paddle.VerifySignature(body, r.Header.Get(paddle.SignatureHeader), secret); err != nil {
//		// reject — invalid signature or replayed delivery
//	}
//	evt, err := paddle.ParseEvent(body)
//	// ...
//	switch evt.EventType {
//	case paddle.EventTransactionCompleted:
//		txn, _ := evt.Transaction()
//	case paddle.EventSubscriptionUpdated:
//		sub, _ := evt.Subscription()
//	}
//
// Use NewVerifier when the default timestamp tolerance
// (DefaultTimestampTolerance) needs tightening, and evt.UserID() to recover the
// internal user ID carried in custom_data.
//
// # Checkout redirect URLs
//
// Paddle Billing transactions have no success_url or cancel_url fields —
// post-checkout redirects are configured client-side via Paddle.js
// (settings.successUrl). CreateTransaction accepts SuccessURL and CancelURL
// and carries them to the Paddle.js checkout page as query parameters on
// CheckoutURL, which is the only server-side channel available.
//
// This package is a provider library only: nothing here runs until it is
// wired into a service context and configured with real Paddle credentials.
package paddle
