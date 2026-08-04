package logic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ============================================
// Validation tests
//
// These tests verify the input validation paths in each logic function.
// They pass invalid inputs that cause the function to return an error
// BEFORE accessing any external dependencies (Repo, TokenMaker, Redis, etc.),
// so a zero-value ServiceContext is safe.
// ============================================

// newTestLogicSvcCtx returns a minimal ServiceContext with all deps nil/zero.
// This is safe for validation tests because the validation paths return early
// before touching any dep.
func newTestLogicSvcCtx() *svc.ServiceContext {
	return &svc.ServiceContext{}
}

func assertGrpcError(t *testing.T, err error, expectedCode codes.Code, expectedMsg string) {
	t.Helper()
	requireGrpcError(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok, "error should be a gRPC status error")
	assert.Equal(t, expectedCode, st.Code())
	assert.Equal(t, expectedMsg, st.Message())
}

func requireGrpcError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

// ============================================
// LoginLogic validation
// ============================================

func TestLoginLogic_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("returns InvalidArgument for nil request", func(t *testing.T) {
		l := NewLoginLogic(ctx, newTestLogicSvcCtx())
		_, err := l.Login(nil)
		assertGrpcError(t, err, codes.InvalidArgument, MsgEmailAndPasswordRequired)
	})

	t.Run("returns InvalidArgument for empty email", func(t *testing.T) {
		l := NewLoginLogic(ctx, newTestLogicSvcCtx())
		_, err := l.Login(&auth.LoginRequest{Email: "", Password: "password123"})
		assertGrpcError(t, err, codes.InvalidArgument, MsgEmailAndPasswordRequired)
	})

	t.Run("returns InvalidArgument for empty password", func(t *testing.T) {
		l := NewLoginLogic(ctx, newTestLogicSvcCtx())
		_, err := l.Login(&auth.LoginRequest{Email: "user@example.com", Password: ""})
		assertGrpcError(t, err, codes.InvalidArgument, MsgEmailAndPasswordRequired)
	})

	t.Run("returns InvalidArgument for invalid email format", func(t *testing.T) {
		l := NewLoginLogic(ctx, newTestLogicSvcCtx())
		_, err := l.Login(&auth.LoginRequest{Email: "notanemail", Password: "password123"})
		assertGrpcError(t, err, codes.InvalidArgument, MsgInvalidEmailFormat)
	})
}

// ============================================
// RegisterLogic validation
// ============================================

func TestRegisterLogic_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("returns InvalidArgument for nil request", func(t *testing.T) {
		l := NewRegisterLogic(ctx, newTestLogicSvcCtx())
		_, err := l.Register(nil)
		assertGrpcError(t, err, codes.InvalidArgument, MsgRequestIsNil)
	})

	t.Run("returns InvalidArgument for empty username", func(t *testing.T) {
		l := NewRegisterLogic(ctx, newTestLogicSvcCtx())
		_, err := l.Register(&auth.RegisterRequest{
			Username: "",
			Email:    "jane@example.com",
			Password: "Abcdef1!",
		})
		assertGrpcError(t, err, codes.InvalidArgument, MsgUsernameEmailPasswordReq)
	})

	t.Run("returns InvalidArgument for empty email", func(t *testing.T) {
		l := NewRegisterLogic(ctx, newTestLogicSvcCtx())
		_, err := l.Register(&auth.RegisterRequest{
			Username: "janedoe",
			Email:    "",
			Password: "Abcdef1!",
		})
		assertGrpcError(t, err, codes.InvalidArgument, MsgUsernameEmailPasswordReq)
	})

	t.Run("returns InvalidArgument for empty password", func(t *testing.T) {
		l := NewRegisterLogic(ctx, newTestLogicSvcCtx())
		_, err := l.Register(&auth.RegisterRequest{
			Username: "janedoe",
			Email:    "jane@example.com",
			Password: "",
		})
		assertGrpcError(t, err, codes.InvalidArgument, MsgUsernameEmailPasswordReq)
	})

	t.Run("returns InvalidArgument for invalid username format", func(t *testing.T) {
		l := NewRegisterLogic(ctx, newTestLogicSvcCtx())
		_, err := l.Register(&auth.RegisterRequest{
			Username: "JaneDoe", // uppercase not allowed
			Email:    "jane@example.com",
			Password: "Abcdef1!",
		})
		assertGrpcError(t, err, codes.InvalidArgument, MsgUsernameFormat)
	})

	t.Run("returns InvalidArgument for invalid email format", func(t *testing.T) {
		l := NewRegisterLogic(ctx, newTestLogicSvcCtx())
		_, err := l.Register(&auth.RegisterRequest{
			Username: "janedoe",
			Email:    "notanemail",
			Password: "Abcdef1!",
		})
		assertGrpcError(t, err, codes.InvalidArgument, MsgInvalidEmailFormat)
	})

	t.Run("returns InvalidArgument for weak password (no uppercase)", func(t *testing.T) {
		l := NewRegisterLogic(ctx, newTestLogicSvcCtx())
		_, err := l.Register(&auth.RegisterRequest{
			Username: "janedoe",
			Email:    "jane@example.com",
			Password: "alllowercase1!",
		})
		assertGrpcError(t, err, codes.InvalidArgument, MsgPasswordStrength)
	})

	t.Run("returns InvalidArgument for weak password (no special char)", func(t *testing.T) {
		l := NewRegisterLogic(ctx, newTestLogicSvcCtx())
		_, err := l.Register(&auth.RegisterRequest{
			Username: "janedoe",
			Email:    "jane@example.com",
			Password: "Nospecialchar1",
		})
		assertGrpcError(t, err, codes.InvalidArgument, MsgPasswordStrength)
	})

	t.Run("returns InvalidArgument for weak password (too short)", func(t *testing.T) {
		l := NewRegisterLogic(ctx, newTestLogicSvcCtx())
		_, err := l.Register(&auth.RegisterRequest{
			Username: "janedoe",
			Email:    "jane@example.com",
			Password: "Ab1!",
		})
		assertGrpcError(t, err, codes.InvalidArgument, MsgPasswordStrength)
	})
}

// ============================================
// VerifyEmailLogic validation
// ============================================

func TestVerifyEmailLogic_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("returns InvalidArgument for nil request", func(t *testing.T) {
		l := NewVerifyEmailLogic(ctx, newTestLogicSvcCtx())
		_, err := l.VerifyEmail(nil)
		assertGrpcError(t, err, codes.InvalidArgument, MsgTokenIsRequired)
	})

	t.Run("returns InvalidArgument for empty token", func(t *testing.T) {
		l := NewVerifyEmailLogic(ctx, newTestLogicSvcCtx())
		_, err := l.VerifyEmail(&auth.VerifyEmailRequest{Token: ""})
		assertGrpcError(t, err, codes.InvalidArgument, MsgTokenIsRequired)
	})
}

// ============================================
// ResetPasswordLogic validation
// ============================================

func TestResetPasswordLogic_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("returns InvalidArgument for nil request", func(t *testing.T) {
		l := NewResetPasswordLogic(ctx, newTestLogicSvcCtx())
		_, err := l.ResetPassword(nil)
		assertGrpcError(t, err, codes.InvalidArgument, MsgTokenAndPasswordRequired)
	})

	t.Run("returns InvalidArgument for empty token", func(t *testing.T) {
		l := NewResetPasswordLogic(ctx, newTestLogicSvcCtx())
		_, err := l.ResetPassword(&auth.ResetPasswordRequest{Token: "", NewPassword: "Abcdef1!"})
		assertGrpcError(t, err, codes.InvalidArgument, MsgTokenAndPasswordRequired)
	})

	t.Run("returns InvalidArgument for empty new password", func(t *testing.T) {
		l := NewResetPasswordLogic(ctx, newTestLogicSvcCtx())
		_, err := l.ResetPassword(&auth.ResetPasswordRequest{Token: "valid-token", NewPassword: ""})
		assertGrpcError(t, err, codes.InvalidArgument, MsgTokenAndPasswordRequired)
	})
}

// ============================================
// ForgotPasswordLogic validation
// ============================================

func TestForgotPasswordLogic_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("returns InvalidArgument for nil request", func(t *testing.T) {
		l := NewForgotPasswordLogic(ctx, newTestLogicSvcCtx())
		_, err := l.ForgotPassword(nil)
		assertGrpcError(t, err, codes.InvalidArgument, MsgEmailIsRequired)
	})

	t.Run("returns InvalidArgument for empty email", func(t *testing.T) {
		l := NewForgotPasswordLogic(ctx, newTestLogicSvcCtx())
		_, err := l.ForgotPassword(&auth.ForgotPasswordRequest{Email: ""})
		assertGrpcError(t, err, codes.InvalidArgument, MsgEmailIsRequired)
	})
}

// ============================================
// ResendVerificationLogic validation
// ============================================

func TestResendVerificationLogic_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("returns InvalidArgument for nil request", func(t *testing.T) {
		l := NewResendVerificationLogic(ctx, newTestLogicSvcCtx())
		_, err := l.ResendVerification(nil)
		assertGrpcError(t, err, codes.InvalidArgument, MsgEmailIsRequired)
	})

	t.Run("returns InvalidArgument for empty email", func(t *testing.T) {
		l := NewResendVerificationLogic(ctx, newTestLogicSvcCtx())
		_, err := l.ResendVerification(&auth.ResendVerificationRequest{Email: ""})
		assertGrpcError(t, err, codes.InvalidArgument, MsgEmailIsRequired)
	})

	t.Run("returns InvalidArgument for invalid email format", func(t *testing.T) {
		l := NewResendVerificationLogic(ctx, newTestLogicSvcCtx())
		_, err := l.ResendVerification(&auth.ResendVerificationRequest{Email: "notanemail"})
		assertGrpcError(t, err, codes.InvalidArgument, MsgInvalidEmailFormat)
	})
}

// ============================================
// LogoutLogic validation
// ============================================

func TestLogoutLogic_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("returns InvalidArgument for nil request", func(t *testing.T) {
		l := NewLogoutLogic(ctx, newTestLogicSvcCtx())
		_, err := l.Logout(nil)
		assertGrpcError(t, err, codes.InvalidArgument, MsgAccessTokenRequired)
	})

	t.Run("returns InvalidArgument for empty access token", func(t *testing.T) {
		l := NewLogoutLogic(ctx, newTestLogicSvcCtx())
		_, err := l.Logout(&auth.LogoutRequest{AccessToken: ""})
		assertGrpcError(t, err, codes.InvalidArgument, MsgAccessTokenRequired)
	})
}

// ============================================
// RefreshTokenLogic validation
// ============================================

func TestRefreshTokenLogic_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("returns InvalidArgument for nil request", func(t *testing.T) {
		l := NewRefreshTokenLogic(ctx, newTestLogicSvcCtx())
		_, err := l.RefreshToken(nil)
		assertGrpcError(t, err, codes.InvalidArgument, MsgRefreshTokenRequired)
	})

	t.Run("returns InvalidArgument for empty refresh token", func(t *testing.T) {
		l := NewRefreshTokenLogic(ctx, newTestLogicSvcCtx())
		_, err := l.RefreshToken(&auth.RefreshRequest{RefreshToken: ""})
		assertGrpcError(t, err, codes.InvalidArgument, MsgRefreshTokenRequired)
	})
}
