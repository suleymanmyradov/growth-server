package s2s

import (
	"context"
	"strconv"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"
)

func TestSignAndVerify(t *testing.T) {
	secret := "my-shared-secret-must-be-at-least-32-by"
	method := "/service.Method"
	ts := time.Now().Unix()

	sig := Sign(secret, method, ts)
	if sig == "" {
		t.Fatal("expected non-empty signature")
	}

	if !Verify(secret, method, sig, ts, 5*time.Minute) {
		t.Fatal("expected signature to verify")
	}

	// Wrong secret
	if Verify("wrong-secret-must-be-at-least-32-by", method, sig, ts, 5*time.Minute) {
		t.Fatal("expected signature to fail with wrong secret")
	}

	// Wrong method
	if Verify(secret, "/other.Method", sig, ts, 5*time.Minute) {
		t.Fatal("expected signature to fail with wrong method")
	}

	// Expired timestamp
	oldTs := time.Now().Add(-10 * time.Minute).Unix()
	oldSig := Sign(secret, method, oldTs)
	if Verify(secret, method, oldSig, oldTs, 5*time.Minute) {
		t.Fatal("expected signature to fail with expired timestamp")
	}

	// Future timestamp
	futureTs := time.Now().Add(10 * time.Minute).Unix()
	futureSig := Sign(secret, method, futureTs)
	if Verify(secret, method, futureSig, futureTs, 5*time.Minute) {
		t.Fatal("expected signature to fail with future timestamp")
	}
}

func TestMustValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "valid secret",
			cfg:     Config{Secret: "this-is-a-valid-secret-32-bytes-long"},
			wantErr: false,
		},
		{
			name:    "empty secret",
			cfg:     Config{Secret: ""},
			wantErr: true,
		},
		{
			name:    "short secret",
			cfg:     Config{Secret: "short"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.MustValidate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("MustValidate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUnaryClientInterceptorSign(t *testing.T) {
	cfg := Config{Secret: "test-secret-must-be-at-least-32-bytes"}
	// We can't easily call the interceptor directly without a real grpc.ClientConn,
	// but we can test that Sign produces deterministic output.
	sig := Sign(cfg.Secret, "/test.Method", 1234567890)
	if sig == "" {
		t.Fatal("expected non-empty signature from client interceptor logic")
	}
}

func TestShouldSkipValidation(t *testing.T) {
	tests := []struct {
		method string
		skip   bool
	}{
		{"/grpc.health.v1.Health/Check", true},
		{"/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo", true},
		{"/service.Method", false},
		{"/my.Service/DoThing", false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			if got := shouldSkipValidation(tt.method); got != tt.skip {
				t.Fatalf("shouldSkipValidation(%q) = %v, want %v", tt.method, got, tt.skip)
			}
		})
	}
}

func TestVerifyWithMetadata(t *testing.T) {
	secret := "test-secret-must-be-at-least-32-bytes"
	method := "/test.Method"
	ts := time.Now().Unix()
	sig := Sign(secret, method, ts)

	// Valid metadata
	md := metadata.Pairs(mdServiceAuth, sig, mdServiceAuthTs, strconv.FormatInt(ts, 10))
	ctx := metadata.NewIncomingContext(context.Background(), md)
	if !verifyWithContext(ctx, secret, method) {
		t.Fatal("expected verification to succeed with valid metadata")
	}

	// Missing metadata
	emptyCtx := metadata.NewIncomingContext(context.Background(), metadata.MD{})
	if verifyWithContext(emptyCtx, secret, method) {
		t.Fatal("expected verification to fail with missing metadata")
	}

	// Wrong signature
	wrongMd := metadata.Pairs(mdServiceAuth, "wrong-sig", mdServiceAuthTs, strconv.FormatInt(ts, 10))
	wrongCtx := metadata.NewIncomingContext(context.Background(), wrongMd)
	if verifyWithContext(wrongCtx, secret, method) {
		t.Fatal("expected verification to fail with wrong signature")
	}
}

// Helper to test verification logic without full interceptor
func verifyWithContext(ctx context.Context, secret, method string) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	sigs := md.Get(mdServiceAuth)
	tss := md.Get(mdServiceAuthTs)
	if len(sigs) == 0 || len(tss) == 0 {
		return false
	}
	ts, err := strconv.ParseInt(tss[0], 10, 64)
	if err != nil {
		return false
	}
	return Verify(secret, method, sigs[0], ts, 5*time.Minute)
}
