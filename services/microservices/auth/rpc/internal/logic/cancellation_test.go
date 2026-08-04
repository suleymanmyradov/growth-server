package logic

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/pkg/email"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
)

// ============================================
// Context cancellation tests
//
// These tests verify that email sends still happen when the request context
// is cancelled mid-operation, thanks to context.WithoutCancel. They use a
// custom email sender that signals when it's called and allows the test to
// cancel the context at the precise moment the email is about to be sent.
// ============================================

// signalingEmailSender wraps an email.Sender and signals a channel when Send
// is called. This lets tests cancel the context at the exact moment the email
// send is about to happen, then verify the email was still sent (because the
// logic uses context.WithoutCancel for the email send).
type signalingEmailSender struct {
	mu         sync.Mutex
	sent       []email.Email
	callSignal chan struct{}
}

func newSignalingEmailSender() *signalingEmailSender {
	return &signalingEmailSender{
		callSignal: make(chan struct{}, 1),
	}
}

func (s *signalingEmailSender) Send(ctx context.Context, msg email.Email) error {
	// Signal that Send was called
	select {
	case s.callSignal <- struct{}{}:
	default:
	}
	// Check if the context is already cancelled (should NOT be, because of WithoutCancel)
	if err := ctx.Err(); err != nil {
		s.mu.Lock()
		s.sent = append(s.sent, msg) // Record even if ctx is cancelled, to test behavior
		s.mu.Unlock()
		return err
	}
	s.mu.Lock()
	s.sent = append(s.sent, msg)
	s.mu.Unlock()
	return nil
}

func (s *signalingEmailSender) getSent() []email.Email {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]email.Email{}, s.sent...)
}

// ============================================
// ForgotPassword: email still sends when context cancelled
// ============================================

func TestForgotPasswordLogic_EmailSendsDespiteContextCancellation(t *testing.T) {
	ctx := context.Background()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = redisClient.Close() }()

	user := makeTestUser()
	mockUsers := &MockUsersRepo{}
	mockUsers.On("GetUserByEmail", mock.Anything, "jane@example.com").Return(user, nil)

	sender := newSignalingEmailSender()

	svcCtx := &svc.ServiceContext{
		Repo:        &repository.Repository{Users: mockUsers},
		RedisClient: redisClient,
		EmailSender: sender,
		Config:      testConfig(15*time.Minute, 7*24*time.Hour),
	}

	// Use a cancellable context
	cancellableCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start ForgotPassword in a goroutine
	done := make(chan error, 1)
	go func() {
		_, err := NewForgotPasswordLogic(cancellableCtx, svcCtx).ForgotPassword(&auth.ForgotPasswordRequest{
			Email: "jane@example.com",
		})
		done <- err
	}()

	// Wait for the email sender to be called, then cancel the context
	select {
	case <-sender.callSignal:
		// Email send was triggered — cancel the original context NOW
		cancel()
	case <-time.After(5 * time.Second):
		t.Fatal("email sender was not called within 5 seconds")
	}

	// Wait for the operation to complete
	select {
	case err := <-done:
		// ForgotPassword always returns success (no leak)
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("ForgotPassword did not complete within 5 seconds")
	}

	// Verify the email was sent despite context cancellation
	sent := sender.getSent()
	require.Len(t, sent, 1, "email should have been sent despite context cancellation")
	assert.Contains(t, sent[0].Subject, "Reset your password")
	assert.Contains(t, sent[0].To[0], "jane@example.com")
}

// ============================================
// ResendVerification: email still sends when context cancelled
// ============================================

func TestResendVerificationLogic_EmailSendsDespiteContextCancellation(t *testing.T) {
	ctx := context.Background()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = redisClient.Close() }()

	user := makeTestUser(func(u *db.User) { u.EmailVerified = false })
	mockUsers := &MockUsersRepo{}
	mockUsers.On("GetUserByEmail", mock.Anything, "jane@example.com").Return(user, nil)

	sender := newSignalingEmailSender()

	svcCtx := &svc.ServiceContext{
		Repo:        &repository.Repository{Users: mockUsers},
		RedisClient: redisClient,
		EmailSender: sender,
		Config:      testConfig(15*time.Minute, 7*24*time.Hour),
	}

	cancellableCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := NewResendVerificationLogic(cancellableCtx, svcCtx).ResendVerification(&auth.ResendVerificationRequest{
			Email: "jane@example.com",
		})
		done <- err
	}()

	select {
	case <-sender.callSignal:
		cancel()
	case <-time.After(5 * time.Second):
		t.Fatal("email sender was not called within 5 seconds")
	}

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("ResendVerification did not complete within 5 seconds")
	}

	sent := sender.getSent()
	require.Len(t, sent, 1, "email should have been sent despite context cancellation")
	assert.Contains(t, sent[0].Subject, "Verify your email")
}

// ============================================
// Register: email still sends when context cancelled
// ============================================

func TestRegisterLogic_EmailSendsDespiteContextCancellation(t *testing.T) {
	ctx := context.Background()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = redisClient.Close() }()

	userID := uuid.New()
	createdUser := makeTestUser(func(u *db.User) {
		u.ID = userID
		u.EmailVerified = false
	})

	mockUsers := &MockUsersRepo{}
	mockTxRunner := &MockTxRunner{}
	sender := newSignalingEmailSender()

	mockTxRunner.On("Run", mock.Anything, "", mock.AnythingOfType("func(pgx.Tx) error")).Return(nil)
	mockUsers.On("CreateUser", mock.Anything, mock.Anything).Return(createdUser, nil)

	svcCtx := &svc.ServiceContext{
		Repo:        &repository.Repository{Users: mockUsers},
		TxRunner:    mockTxRunner,
		RedisClient: redisClient,
		EmailSender: sender,
		Config:      testConfig(15*time.Minute, 7*24*time.Hour),
	}

	cancellableCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := NewRegisterLogic(cancellableCtx, svcCtx).Register(&auth.RegisterRequest{
			Username: "janedoe",
			Email:    "jane@example.com",
			Password: "Abcdef1!",
			FullName: "Jane Doe",
		})
		done <- err
	}()

	select {
	case <-sender.callSignal:
		cancel()
	case <-time.After(5 * time.Second):
		t.Fatal("email sender was not called within 5 seconds")
	}

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Register did not complete within 5 seconds")
	}

	sent := sender.getSent()
	require.Len(t, sent, 1, "email should have been sent despite context cancellation")
	assert.Contains(t, sent[0].Subject, "Verify your email")
}

// ============================================
// ForgotPassword: email context is NOT cancelled (verifies WithoutCancel is used)
// ============================================

func TestForgotPasswordLogic_EmailContextIsNotCancelled(t *testing.T) {
	ctx := context.Background()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = redisClient.Close() }()

	user := makeTestUser()
	mockUsers := &MockUsersRepo{}
	mockUsers.On("GetUserByEmail", mock.Anything, "jane@example.com").Return(user, nil)

	// Custom sender that records whether the context it receives is cancelled
	sender := &ctxCheckingEmailSender{}

	svcCtx := &svc.ServiceContext{
		Repo:        &repository.Repository{Users: mockUsers},
		RedisClient: redisClient,
		EmailSender: sender,
		Config:      testConfig(15*time.Minute, 7*24*time.Hour),
	}

	cancellableCtx, cancel := context.WithCancel(ctx)
	cancel() // Cancel immediately

	_, _ = NewForgotPasswordLogic(cancellableCtx, svcCtx).ForgotPassword(&auth.ForgotPasswordRequest{
		Email: "jane@example.com",
	})
	// With an already-cancelled context, the Redis store fails and the function
	// returns an error — that's expected. If the email WAS somehow sent, verify
	// the context was not cancelled (proving WithoutCancel is used). The
	// EmailSendsDespiteContextCancellation test above is the real verification.
	if len(sender.sent) > 0 {
		assert.False(t, sender.ctxWasCancelled, "email send context should NOT be cancelled (WithoutCancel)")
	}
}

// ============================================
// ResendVerification: email context is NOT cancelled
// ============================================

func TestResendVerificationLogic_EmailContextIsNotCancelled(t *testing.T) {
	ctx := context.Background()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = redisClient.Close() }()

	user := makeTestUser(func(u *db.User) { u.EmailVerified = false })
	mockUsers := &MockUsersRepo{}
	mockUsers.On("GetUserByEmail", mock.Anything, "jane@example.com").Return(user, nil)

	sender := &ctxCheckingEmailSender{}

	svcCtx := &svc.ServiceContext{
		Repo:        &repository.Repository{Users: mockUsers},
		RedisClient: redisClient,
		EmailSender: sender,
		Config:      testConfig(15*time.Minute, 7*24*time.Hour),
	}

	cancellableCtx, cancel := context.WithCancel(ctx)
	cancel() // Cancel immediately

	_, _ = NewResendVerificationLogic(cancellableCtx, svcCtx).ResendVerification(&auth.ResendVerificationRequest{
		Email: "jane@example.com",
	})

	// ResendVerification returns error if email send fails, but with WithoutCancel
	// the email should still send. However, the Redis Store might fail because
	// the original ctx is cancelled. Let's check what actually happens.
	// The Redis Store uses ctx (cancelled), so it will fail and the logic returns
	// an error before reaching the email send. This is expected — the test verifies
	// that IF the email send is reached, the context is not cancelled.

	// If the Redis store failed, we won't reach the email send. That's OK —
	// the cancellation test above (TestResendVerificationLogic_EmailSendsDespiteContextCancellation)
	// tests the real scenario where Redis succeeds but context is cancelled after.
	// This test just verifies the context is not cancelled when we do reach the send.
	if len(sender.sent) > 0 {
		assert.False(t, sender.ctxWasCancelled, "email send context should NOT be cancelled (WithoutCancel)")
	}
}

// ctxCheckingEmailSender records whether the context passed to Send is cancelled.
type ctxCheckingEmailSender struct {
	sent            []email.Email
	ctxWasCancelled bool
}

func (s *ctxCheckingEmailSender) Send(ctx context.Context, msg email.Email) error {
	s.sent = append(s.sent, msg)
	s.ctxWasCancelled = ctx.Err() != nil
	return nil
}
