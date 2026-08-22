package email

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestResendSender_IdempotencyKey(t *testing.T) {
	sender := NewResendSender(Config{APIKey: "test", FromAddress: "test@example.com"})
	sender.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("Idempotency-Key"); got != "delivery-1" {
			t.Fatalf("expected idempotency key, got %q", got)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	})}

	err := sender.Send(context.Background(), Email{
		To:             []string{"user@example.com"},
		Subject:        "Subject",
		HTML:           "<p>Body</p>",
		IdempotencyKey: "delivery-1",
	})
	if err != nil {
		t.Fatalf("send email: %v", err)
	}
}
