package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ============================================
// Gateway auth logic validation tests
//
// These tests verify the input validation paths in the gateway auth logic
// functions. The validation returns an error before accessing svcCtx.AuthRpc,
// so a nil ServiceContext is safe.
// ============================================

func assertGrpcError(t *testing.T, err error, expectedCode codes.Code, expectedMsg string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	st, ok := status.FromError(err)
	assert.True(t, ok, "error should be a gRPC status error")
	assert.Equal(t, expectedCode, st.Code())
	assert.Equal(t, expectedMsg, st.Message())
}

// ============================================
// LoginLogic validation
// ============================================

func TestGatewayLoginLogic_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("returns InvalidArgument for empty email", func(t *testing.T) {
		l := NewLoginLogic(ctx, &svc.ServiceContext{})
		_, err := l.Login(&types.LoginRequest{Email: "", Password: "password123"})
		assertGrpcError(t, err, codes.InvalidArgument, MsgValidEmailRequired)
	})

	t.Run("returns InvalidArgument for invalid email format", func(t *testing.T) {
		l := NewLoginLogic(ctx, &svc.ServiceContext{})
		_, err := l.Login(&types.LoginRequest{Email: "notanemail", Password: "password123"})
		assertGrpcError(t, err, codes.InvalidArgument, MsgValidEmailRequired)
	})

	t.Run("returns InvalidArgument for empty password", func(t *testing.T) {
		l := NewLoginLogic(ctx, &svc.ServiceContext{})
		_, err := l.Login(&types.LoginRequest{Email: "user@example.com", Password: ""})
		assertGrpcError(t, err, codes.InvalidArgument, MsgPasswordRequired)
	})
}

// ============================================
// RegisterLogic validation
// ============================================

func TestGatewayRegisterLogic_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("returns InvalidArgument for empty username", func(t *testing.T) {
		l := NewRegisterLogic(ctx, &svc.ServiceContext{})
		_, err := l.Register(&types.RegisterRequest{
			Username: "",
			Email:    "jane@example.com",
			Password: "Abcdef1!",
			FullName: "Jane Doe",
		})
		assertGrpcError(t, err, codes.InvalidArgument, MsgUsernameRequired)
	})

	t.Run("returns InvalidArgument for invalid username format", func(t *testing.T) {
		l := NewRegisterLogic(ctx, &svc.ServiceContext{})
		_, err := l.Register(&types.RegisterRequest{
			Username: "JaneDoe", // uppercase not allowed
			Email:    "jane@example.com",
			Password: "Abcdef1!",
			FullName: "Jane Doe",
		})
		assertGrpcError(t, err, codes.InvalidArgument, MsgUsernameFormat)
	})

	t.Run("returns InvalidArgument for empty email", func(t *testing.T) {
		l := NewRegisterLogic(ctx, &svc.ServiceContext{})
		_, err := l.Register(&types.RegisterRequest{
			Username: "janedoe",
			Email:    "",
			Password: "Abcdef1!",
			FullName: "Jane Doe",
		})
		assertGrpcError(t, err, codes.InvalidArgument, MsgValidEmailRequired)
	})

	t.Run("returns InvalidArgument for invalid email format", func(t *testing.T) {
		l := NewRegisterLogic(ctx, &svc.ServiceContext{})
		_, err := l.Register(&types.RegisterRequest{
			Username: "janedoe",
			Email:    "notanemail",
			Password: "Abcdef1!",
			FullName: "Jane Doe",
		})
		assertGrpcError(t, err, codes.InvalidArgument, MsgValidEmailRequired)
	})

	t.Run("returns InvalidArgument for weak password", func(t *testing.T) {
		l := NewRegisterLogic(ctx, &svc.ServiceContext{})
		_, err := l.Register(&types.RegisterRequest{
			Username: "janedoe",
			Email:    "jane@example.com",
			Password: "alllowercase",
			FullName: "Jane Doe",
		})
		assertGrpcError(t, err, codes.InvalidArgument, MsgPasswordStrength)
	})

	t.Run("returns InvalidArgument for empty full name", func(t *testing.T) {
		l := NewRegisterLogic(ctx, &svc.ServiceContext{})
		_, err := l.Register(&types.RegisterRequest{
			Username: "janedoe",
			Email:    "jane@example.com",
			Password: "Abcdef1!",
			FullName: "",
		})
		assertGrpcError(t, err, codes.InvalidArgument, MsgFullNameRequired)
	})
}

// ============================================
// VerifyEmailLogic validation
// ============================================

func TestGatewayVerifyEmailLogic_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("returns InvalidArgument for nil request", func(t *testing.T) {
		l := NewVerifyEmailLogic(ctx, &svc.ServiceContext{})
		_, err := l.VerifyEmail(nil)
		assertGrpcError(t, err, codes.InvalidArgument, MsgTokenRequired)
	})

	t.Run("returns InvalidArgument for empty token", func(t *testing.T) {
		l := NewVerifyEmailLogic(ctx, &svc.ServiceContext{})
		_, err := l.VerifyEmail(&types.VerifyEmailRequest{Token: ""})
		assertGrpcError(t, err, codes.InvalidArgument, MsgTokenRequired)
	})
}

// ============================================
// ResetPasswordLogic validation
// ============================================

func TestGatewayResetPasswordLogic_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("returns InvalidArgument for nil request", func(t *testing.T) {
		l := NewResetPasswordLogic(ctx, &svc.ServiceContext{})
		_, err := l.ResetPassword(nil)
		assertGrpcError(t, err, codes.InvalidArgument, MsgTokenAndStrongPasswordReq)
	})

	t.Run("returns InvalidArgument for empty token", func(t *testing.T) {
		l := NewResetPasswordLogic(ctx, &svc.ServiceContext{})
		_, err := l.ResetPassword(&types.ResetPasswordRequest{Token: "", NewPassword: "Abcdef1!"})
		assertGrpcError(t, err, codes.InvalidArgument, MsgTokenAndStrongPasswordReq)
	})

	t.Run("returns InvalidArgument for weak password", func(t *testing.T) {
		l := NewResetPasswordLogic(ctx, &svc.ServiceContext{})
		_, err := l.ResetPassword(&types.ResetPasswordRequest{Token: "valid-token", NewPassword: "weak"})
		assertGrpcError(t, err, codes.InvalidArgument, MsgTokenAndStrongPasswordReq)
	})
}

// ============================================
// ForgotPasswordLogic validation
// ============================================

func TestGatewayForgotPasswordLogic_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("returns InvalidArgument for nil request", func(t *testing.T) {
		l := NewForgotPasswordLogic(ctx, &svc.ServiceContext{})
		_, err := l.ForgotPassword(nil)
		assertGrpcError(t, err, codes.InvalidArgument, MsgValidEmailRequired)
	})

	t.Run("returns InvalidArgument for invalid email format", func(t *testing.T) {
		l := NewForgotPasswordLogic(ctx, &svc.ServiceContext{})
		_, err := l.ForgotPassword(&types.ForgotPasswordRequest{Email: "notanemail"})
		assertGrpcError(t, err, codes.InvalidArgument, MsgValidEmailRequired)
	})
}

// ============================================
// ResendVerificationLogic validation
// ============================================

func TestGatewayResendVerificationLogic_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("returns InvalidArgument for nil request", func(t *testing.T) {
		l := NewResendVerificationLogic(ctx, &svc.ServiceContext{})
		_, err := l.ResendVerification(nil)
		assertGrpcError(t, err, codes.InvalidArgument, MsgValidEmailRequired)
	})

	t.Run("returns InvalidArgument for invalid email format", func(t *testing.T) {
		l := NewResendVerificationLogic(ctx, &svc.ServiceContext{})
		_, err := l.ResendVerification(&types.ResendVerificationRequest{Email: "notanemail"})
		assertGrpcError(t, err, codes.InvalidArgument, MsgValidEmailRequired)
	})
}

// ============================================
// GoogleLoginLogic validation
// ============================================

func TestGatewayGoogleLoginLogic_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("returns InvalidArgument for nil request", func(t *testing.T) {
		l := NewGoogleLoginLogic(ctx, &svc.ServiceContext{})
		_, err := l.GoogleLogin(nil)
		assertGrpcError(t, err, codes.InvalidArgument, MsgAuthorizationCodeRequired)
	})

	t.Run("returns InvalidArgument for empty authorization code", func(t *testing.T) {
		l := NewGoogleLoginLogic(ctx, &svc.ServiceContext{})
		_, err := l.GoogleLogin(&types.GoogleLoginRequest{AuthorizationCode: ""})
		assertGrpcError(t, err, codes.InvalidArgument, MsgAuthorizationCodeRequired)
	})
}

// ============================================
// AppleLoginLogic validation
// ============================================

func TestGatewayAppleLoginLogic_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("returns InvalidArgument for nil request", func(t *testing.T) {
		l := NewAppleLoginLogic(ctx, &svc.ServiceContext{})
		_, err := l.AppleLogin(nil)
		assertGrpcError(t, err, codes.InvalidArgument, MsgIdentityTokenRequired)
	})

	t.Run("returns InvalidArgument for empty identity token", func(t *testing.T) {
		l := NewAppleLoginLogic(ctx, &svc.ServiceContext{})
		_, err := l.AppleLogin(&types.AppleLoginRequest{IdentityToken: ""})
		assertGrpcError(t, err, codes.InvalidArgument, MsgIdentityTokenRequired)
	})
}
