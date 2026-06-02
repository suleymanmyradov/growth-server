package authz

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeCache is a minimal in-memory cache for testing.
type fakeCache struct {
	data map[string]string
}

func newFakeCache() *fakeCache {
	return &fakeCache{data: make(map[string]string)}
}

func (f *fakeCache) Get(ctx context.Context, key string) (string, error) {
	val, ok := f.data[key]
	if !ok {
		return "", errors.New("not found")
	}
	return val, nil
}

func (f *fakeCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	f.data[key] = value.(string)
	return nil
}

func (f *fakeCache) Del(ctx context.Context, keys ...string) error {
	for _, k := range keys {
		delete(f.data, k)
	}
	return nil
}

func TestCheckerCheckPrincipalActive(t *testing.T) {
	fc := newFakeCache()
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	checker := NewChecker(fc, func(ctx context.Context, id uuid.UUID) (UserStatus, error) {
		if id == userID {
			return StatusActive, nil
		}
		return StatusNotFound, nil
	})

	ctx := principal.WithPrincipal(context.Background(), principal.Principal{UserID: userID.String()})
	if err := checker.CheckPrincipal(ctx); err != nil {
		t.Fatalf("expected no error for active user, got %v", err)
	}
}

func TestCheckerCheckPrincipalMissing(t *testing.T) {
	fc := newFakeCache()
	checker := NewChecker(fc, func(ctx context.Context, id uuid.UUID) (UserStatus, error) {
		return StatusActive, nil
	})

	ctx := context.Background()
	err := checker.CheckPrincipal(ctx)
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated for missing principal, got %v", err)
	}
}

func TestCheckerCheckPrincipalNotFound(t *testing.T) {
	fc := newFakeCache()
	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	checker := NewChecker(fc, func(ctx context.Context, id uuid.UUID) (UserStatus, error) {
		return StatusNotFound, nil
	})

	ctx := principal.WithPrincipal(context.Background(), principal.Principal{UserID: userID.String()})
	err := checker.CheckPrincipal(ctx)
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated for not found user, got %v", err)
	}
}

func TestCheckerCheckPrincipalInactive(t *testing.T) {
	fc := newFakeCache()
	userID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	checker := NewChecker(fc, func(ctx context.Context, id uuid.UUID) (UserStatus, error) {
		return StatusInactive, nil
	})

	ctx := principal.WithPrincipal(context.Background(), principal.Principal{UserID: userID.String()})
	err := checker.CheckPrincipal(ctx)
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for inactive user, got %v", err)
	}
}

func TestCheckerCachesResult(t *testing.T) {
	fc := newFakeCache()
	userID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	callCount := 0

	checker := NewChecker(fc, func(ctx context.Context, id uuid.UUID) (UserStatus, error) {
		callCount++
		return StatusActive, nil
	})

	ctx := principal.WithPrincipal(context.Background(), principal.Principal{UserID: userID.String()})

	// First call should hit lookup
	if err := checker.CheckPrincipal(ctx); err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 lookup call, got %d", callCount)
	}

	// Second call should use cache
	if err := checker.CheckPrincipal(ctx); err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 lookup call (cached), got %d", callCount)
	}
}

func TestCheckerInvalidate(t *testing.T) {
	fc := newFakeCache()
	userID := uuid.MustParse("55555555-5555-5555-5555-555555555555")

	checker := NewChecker(fc, func(ctx context.Context, id uuid.UUID) (UserStatus, error) {
		return StatusActive, nil
	})

	ctx := principal.WithPrincipal(context.Background(), principal.Principal{UserID: userID.String()})
	_ = checker.CheckPrincipal(ctx)

	// Invalidate cache
	if err := checker.Invalidate(ctx, userID); err != nil {
		t.Fatalf("invalidate failed: %v", err)
	}

	// After invalidate, cache should be empty
	key := userStatusPrefix + userID.String()
	if _, ok := fc.data[key]; ok {
		t.Fatal("expected cache entry to be deleted after invalidate")
	}
}

func TestCheckerLookupError(t *testing.T) {
	fc := newFakeCache()
	userID := uuid.MustParse("66666666-6666-6666-6666-666666666666")

	checker := NewChecker(fc, func(ctx context.Context, id uuid.UUID) (UserStatus, error) {
		return StatusUnknown, errors.New("db down")
	})

	ctx := principal.WithPrincipal(context.Background(), principal.Principal{UserID: userID.String()})
	err := checker.CheckPrincipal(ctx)
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Fatalf("expected Internal for lookup error, got %v", err)
	}
}
