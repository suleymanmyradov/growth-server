package s2s

import (
	"context"
	"io"
	"strconv"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const testSecret = "my-shared-secret-must-be-at-least-32-by"

func TestSignAndVerify(t *testing.T) {
	method := "/service.Method"
	bodyHash := "deadbeef"
	ts := time.Now().Unix()

	sig := Sign(testSecret, method, bodyHash, ts)
	if sig == "" {
		t.Fatal("expected non-empty signature")
	}

	if !Verify(testSecret, method, bodyHash, sig, ts, 5*time.Minute) {
		t.Fatal("expected signature to verify")
	}

	// Wrong secret
	if Verify("wrong-secret-must-be-at-least-32-by", method, bodyHash, sig, ts, 5*time.Minute) {
		t.Fatal("expected signature to fail with wrong secret")
	}

	// Wrong method
	if Verify(testSecret, "/other.Method", bodyHash, sig, ts, 5*time.Minute) {
		t.Fatal("expected signature to fail with wrong method")
	}

	// Wrong body hash — a replayed signature with a different body must fail.
	if Verify(testSecret, method, "cafe", sig, ts, 5*time.Minute) {
		t.Fatal("expected signature to fail with different body hash")
	}

	// Expired timestamp
	oldTs := time.Now().Add(-10 * time.Minute).Unix()
	oldSig := Sign(testSecret, method, bodyHash, oldTs)
	if Verify(testSecret, method, bodyHash, oldSig, oldTs, 5*time.Minute) {
		t.Fatal("expected signature to fail with expired timestamp")
	}

	// Future timestamp
	futureTs := time.Now().Add(10 * time.Minute).Unix()
	futureSig := Sign(testSecret, method, bodyHash, futureTs)
	if Verify(testSecret, method, bodyHash, futureSig, futureTs, 5*time.Minute) {
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

func TestHashRequestDeterministic(t *testing.T) {
	req := wrapperspb.String("hello")
	h1, err := HashRequest(req)
	if err != nil {
		t.Fatalf("HashRequest: %v", err)
	}
	h2, err := HashRequest(wrapperspb.String("hello"))
	if err != nil {
		t.Fatalf("HashRequest: %v", err)
	}
	if h1 != h2 {
		t.Fatal("expected deterministic hash for equal proto messages")
	}
	h3, err := HashRequest(wrapperspb.String("world"))
	if err != nil {
		t.Fatalf("HashRequest: %v", err)
	}
	if h3 == h1 {
		t.Fatal("expected different hash for different message")
	}
}

func TestUnaryInterceptorsBodyHash(t *testing.T) {
	cfg := Config{Secret: testSecret}
	method := "/svc.Test/Method"
	req := wrapperspb.String("payload")

	// Run the client interceptor to produce signed metadata, then feed it to
	// the server interceptor as incoming metadata.
	clientInt := UnaryClientInterceptor(cfg)
	var outgoingMD metadata.MD
	invoker := func(ctx context.Context, _ string, _, _ interface{}, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		md, _ := metadata.FromOutgoingContext(ctx)
		outgoingMD = md
		return nil
	}
	if err := clientInt(context.Background(), method, req, nil, nil, invoker); err != nil {
		t.Fatalf("client interceptor: %v", err)
	}
	if len(outgoingMD.Get(mdServiceAuth)) == 0 || len(outgoingMD.Get(mdServiceAuthBody)) == 0 {
		t.Fatal("expected auth + body hash metadata to be set")
	}

	serverInt := UnaryServerInterceptor(cfg)
	handler := func(ctx context.Context, _ interface{}) (interface{}, error) {
		return "ok", nil
	}
	ctx := metadata.NewIncomingContext(context.Background(), outgoingMD)
	if _, err := serverInt(ctx, req, &grpc.UnaryServerInfo{FullMethod: method}, handler); err != nil {
		t.Fatalf("server interceptor should accept valid signature: %v", err)
	}

	// Same signature, different body → replay must fail.
	if _, err := serverInt(ctx, wrapperspb.String("tampered"), &grpc.UnaryServerInfo{FullMethod: method}, handler); err == nil {
		t.Fatal("expected body hash mismatch to be rejected")
	} else if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}

	// Missing metadata → fail.
	if _, err := serverInt(context.Background(), req, &grpc.UnaryServerInfo{FullMethod: method}, handler); err == nil {
		t.Fatal("expected missing metadata to be rejected")
	}
}

type fakeServerStream struct {
	grpc.ServerStream
	ctx      context.Context
	recvMsgs []interface{}
	err      error
	idx      int
}

func (f *fakeServerStream) Context() context.Context { return f.ctx }

func (f *fakeServerStream) RecvMsg(m interface{}) error {
	if f.idx >= len(f.recvMsgs) {
		return io.EOF
	}
	src := f.recvMsgs[f.idx]
	f.idx++
	if s, ok := src.(*wrapperspb.StringValue); ok {
		if d, ok := m.(*wrapperspb.StringValue); ok {
			d.Value = s.Value
			return nil
		}
	}
	return f.err
}

func TestStreamServerInterceptor(t *testing.T) {
	cfg := Config{Secret: testSecret}
	method := "/svc.Test/StreamMethod"
	req := wrapperspb.String("stream-request")

	// Sign as the client interceptor would with a bound body.
	bodyHash, err := HashRequest(req)
	if err != nil {
		t.Fatalf("HashRequest: %v", err)
	}
	ts := time.Now().Unix()
	sig := Sign(cfg.Secret, method, bodyHash, ts)
	md := metadata.Pairs(
		mdServiceAuth, sig,
		mdServiceAuthTs, strconv.FormatInt(ts, 10),
		mdServiceAuthBody, bodyHash,
	)

	streamInt := StreamServerInterceptor(cfg)
	handler := func(_ interface{}, ss grpc.ServerStream) error {
		msg := &wrapperspb.StringValue{}
		return ss.RecvMsg(msg)
	}

	// Bound body matches → handler's RecvMsg succeeds.
	ss := &fakeServerStream{
		ctx:      metadata.NewIncomingContext(context.Background(), md),
		recvMsgs: []interface{}{wrapperspb.String("stream-request")},
	}
	if err := streamInt(nil, ss, &grpc.StreamServerInfo{FullMethod: method}, handler); err != nil {
		t.Fatalf("expected bound stream to verify: %v", err)
	}

	// Bound body mismatch → RecvMsg fails with PermissionDenied.
	ssBad := &fakeServerStream{
		ctx:      metadata.NewIncomingContext(context.Background(), md),
		recvMsgs: []interface{}{wrapperspb.String("different-request")},
	}
	if err := streamInt(nil, ssBad, &grpc.StreamServerInfo{FullMethod: method}, handler); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied on body mismatch, got %v", err)
	}

	// No signature at all → rejected before the handler runs.
	ssNoAuth := &fakeServerStream{ctx: context.Background()}
	if err := streamInt(nil, ssNoAuth, &grpc.StreamServerInfo{FullMethod: method}, handler); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied without metadata, got %v", err)
	}

	// Unbound signature (empty body hash) → signature verifies, no body check.
	ts2 := time.Now().Unix()
	sig2 := Sign(cfg.Secret, method, "", ts2)
	mdUnbound := metadata.Pairs(mdServiceAuth, sig2, mdServiceAuthTs, strconv.FormatInt(ts2, 10))
	ssUnbound := &fakeServerStream{
		ctx:      metadata.NewIncomingContext(context.Background(), mdUnbound),
		recvMsgs: []interface{}{wrapperspb.String("anything")},
	}
	if err := streamInt(nil, ssUnbound, &grpc.StreamServerInfo{FullMethod: method}, handler); err != nil {
		t.Fatalf("expected unbound stream to verify signature only: %v", err)
	}
}

func TestContextWithSigningBody(t *testing.T) {
	req := wrapperspb.String("x")
	ctx := ContextWithSigningBody(context.Background(), req)
	got, ok := signingBodyFrom(ctx)
	if !ok {
		t.Fatal("expected signing body present")
	}
	if got != req {
		t.Fatal("expected same request object")
	}
	if _, ok := signingBodyFrom(context.Background()); ok {
		t.Fatal("expected no signing body on empty ctx")
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
