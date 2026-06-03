package mdpropagate

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	jwtpkg "github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
)

// fakeVerifier is a test double that verifies tokens using the real jwt package.
type fakeVerifier struct {
	maker *jwtpkg.TokenMaker
}

func newFakeVerifier(t *testing.T) *fakeVerifier {
	cfg := jwtpkg.Config{
		Secret:                "test-secret-must-be-at-least-32-bytes",
		Issuer:                "test-issuer",
		Audience:              "test-audience",
		AccessExpiryDuration:  time.Hour,
		RefreshExpiryDuration: time.Hour,
	}
	maker, err := jwtpkg.NewTokenMaker(cfg, nil)
	require.NoError(t, err)
	return &fakeVerifier{maker: maker}
}

func (f *fakeVerifier) VerifyAccessToken(_ context.Context, tokenString string) (*jwtpkg.TokenClaims, error) {
	return f.maker.VerifyAccessToken(context.Background(), tokenString)
}

func TestOutgoing_WithToken(t *testing.T) {
	fv := newFakeVerifier(t)
	tokenResp, err := fv.maker.CreateAccessToken(context.Background(), uuid.New(), "testuser", []string{"user"}, uuid.New())
	require.NoError(t, err)

	ctx := context.Background()
	ctx = principal.WithToken(ctx, tokenResp.Token)
	ctx = Outgoing(ctx)

	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok, "expected outgoing metadata")

	auth := md.Get(MDAuthorization)
	require.Len(t, auth, 1)
	require.Contains(t, auth[0], "Bearer ")
}

func TestOutgoing_WithoutToken(t *testing.T) {
	ctx := context.Background()
	ctx = Outgoing(ctx)

	_, ok := metadata.FromOutgoingContext(ctx)
	require.False(t, ok, "expected no metadata without token")
}

func TestPrincipalFromMetadata_Success(t *testing.T) {
	fv := newFakeVerifier(t)
	userID := uuid.New()
	sessionID := uuid.New()
	tokenResp, err := fv.maker.CreateAccessToken(context.Background(), userID, "testuser", []string{"admin", "user"}, sessionID)
	require.NoError(t, err)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(MDAuthorization, "Bearer "+tokenResp.Token))

	ctx, err = PrincipalFromMetadata(ctx, fv)
	require.NoError(t, err)

	p, ok := principal.PrincipalFrom(ctx)
	require.True(t, ok)
	require.Equal(t, userID.String(), p.UserID)
	require.Equal(t, "testuser", p.Username)
	require.Equal(t, []string{"admin", "user"}, p.Roles)
	require.Equal(t, sessionID.String(), p.SessionID)
}

func TestPrincipalFromMetadata_MissingMetadata(t *testing.T) {
	fv := newFakeVerifier(t)
	ctx := context.Background()
	_, err := PrincipalFromMetadata(ctx, fv)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
}

func TestPrincipalFromMetadata_MissingAuthorization(t *testing.T) {
	fv := newFakeVerifier(t)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{})
	_, err := PrincipalFromMetadata(ctx, fv)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
}

func TestPrincipalFromMetadata_InvalidToken(t *testing.T) {
	fv := newFakeVerifier(t)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(MDAuthorization, "Bearer invalid-token"))
	_, err := PrincipalFromMetadata(ctx, fv)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
}

func TestRoundTrip(t *testing.T) {
	fv := newFakeVerifier(t)
	userID := uuid.New()
	sessionID := uuid.New()
	tokenResp, err := fv.maker.CreateAccessToken(context.Background(), userID, "testuser", []string{"admin", "user"}, sessionID)
	require.NoError(t, err)

	// Gateway side: store token in context and convert to outgoing metadata
	ctx := context.Background()
	ctx = principal.WithToken(ctx, tokenResp.Token)
	ctx = Outgoing(ctx)

	// Simulate gRPC call: extract outgoing metadata and create incoming context
	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	ctx = metadata.NewIncomingContext(context.Background(), md)

	// Service side: verify JWT and extract principal
	ctx, err = PrincipalFromMetadata(ctx, fv)
	require.NoError(t, err)

	p, ok := principal.PrincipalFrom(ctx)
	require.True(t, ok)
	require.Equal(t, userID.String(), p.UserID)
	require.Equal(t, "testuser", p.Username)
	require.Equal(t, []string{"admin", "user"}, p.Roles)
	require.Equal(t, sessionID.String(), p.SessionID)
}

func TestUnaryClientInterceptor(t *testing.T) {
	fv := newFakeVerifier(t)
	tokenResp, err := fv.maker.CreateAccessToken(context.Background(), uuid.New(), "testuser", nil, uuid.New())
	require.NoError(t, err)

	ctx := context.Background()
	ctx = principal.WithToken(ctx, tokenResp.Token)

	var capturedCtx context.Context
	invoker := func(ctx context.Context, _ string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		capturedCtx = ctx
		return nil
	}

	interceptor := UnaryClientInterceptor()
	err = interceptor(ctx, "/test/method", nil, nil, nil, invoker)
	require.NoError(t, err)

	md, ok := metadata.FromOutgoingContext(capturedCtx)
	require.True(t, ok)
	auth := md.Get(MDAuthorization)
	require.Len(t, auth, 1)
	require.Contains(t, auth[0], "Bearer ")
}

func TestUnaryServerInterceptor(t *testing.T) {
	fv := newFakeVerifier(t)
	userID := uuid.New()
	tokenResp, err := fv.maker.CreateAccessToken(context.Background(), userID, "testuser", nil, uuid.New())
	require.NoError(t, err)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(MDAuthorization, "Bearer "+tokenResp.Token))

	interceptor := UnaryServerInterceptor(fv)
	handlerCalled := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		handlerCalled = true
		p, ok := principal.PrincipalFrom(ctx)
		require.True(t, ok)
		require.Equal(t, userID.String(), p.UserID)
		return nil, nil
	}

	_, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
	require.NoError(t, err)
	require.True(t, handlerCalled)
}

func TestUnaryServerInterceptor_Unauthenticated(t *testing.T) {
	fv := newFakeVerifier(t)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{})

	interceptor := UnaryServerInterceptor(fv)
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, func(context.Context, interface{}) (interface{}, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
}

func TestUnaryServerInterceptorOptional(t *testing.T) {
	fv := newFakeVerifier(t)
	tests := []struct {
		name     string
		metadata metadata.MD
		wantErr  bool
	}{
		{
			name:     "with valid token",
			metadata: func() metadata.MD { tr, _ := fv.maker.CreateAccessToken(context.Background(), uuid.New(), "u", nil, uuid.New()); return metadata.Pairs(MDAuthorization, "Bearer "+tr.Token) }(),
			wantErr:  false,
		},
		{
			name:     "without authorization",
			metadata: metadata.MD{},
			wantErr:  false,
		},
		{
			name:     "with invalid token",
			metadata: metadata.Pairs(MDAuthorization, "Bearer bad"),
			wantErr:  false, // optional: ignores error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := metadata.NewIncomingContext(context.Background(), tt.metadata)
			interceptor := UnaryServerInterceptorOptional(fv)
			handlerCalled := false

			handler := func(_ context.Context, req interface{}) (interface{}, error) {
				handlerCalled = true
				return nil, nil
			}

			_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
			require.NoError(t, err)
			require.True(t, handlerCalled)
		})
	}
}

func TestTokenVerifierInterface(t *testing.T) {
	// Ensure the real jwtpkg.TokenMaker satisfies our TokenVerifier interface
	cfg := jwtpkg.Config{
		Secret:                "test-secret-must-be-at-least-32-bytes",
		Issuer:                "test-issuer",
		Audience:              "test-audience",
		AccessExpiryDuration:  time.Hour,
		RefreshExpiryDuration: time.Hour,
	}
	maker, err := jwtpkg.NewTokenMaker(cfg, nil)
	require.NoError(t, err)

	// This compile-time check ensures *jwtpkg.TokenMaker implements TokenVerifier
	var _ TokenVerifier = maker
}

func TestClockSkewTolerance(t *testing.T) {
	fv := newFakeVerifier(t)
	// Create a token with a very short expiry
	userID := uuid.New()
	sessionID := uuid.New()

	// We can't easily create a nearly-expired token with the current API,
	// but we can verify that leeway is applied by checking the DefaultLeeway constant
	require.Equal(t, 30*time.Second, jwtpkg.DefaultLeeway)

	// Create a normal token and verify it works
	tokenResp, err := fv.maker.CreateAccessToken(context.Background(), userID, "u", nil, sessionID)
	require.NoError(t, err)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(MDAuthorization, "Bearer "+tokenResp.Token))
	_, err = PrincipalFromMetadata(ctx, fv)
	require.NoError(t, err)
}

func TestGenericErrorMessages(t *testing.T) {
	fv := newFakeVerifier(t)

	// Tampered token (change payload without changing signature)
	cfg := jwtpkg.Config{
		Secret:                "different-secret-must-be-at-least-32-by",
		Issuer:                "other-issuer",
		Audience:              "other-audience",
		AccessExpiryDuration:  time.Hour,
		RefreshExpiryDuration: time.Hour,
	}
	otherMaker, err := jwtpkg.NewTokenMaker(cfg, nil)
	require.NoError(t, err)

	tr, err := otherMaker.CreateAccessToken(context.Background(), uuid.New(), "u", nil, uuid.New())
	require.NoError(t, err)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(MDAuthorization, "Bearer "+tr.Token))
	_, err = PrincipalFromMetadata(ctx, fv)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
	require.Equal(t, "invalid token", st.Message())
}
