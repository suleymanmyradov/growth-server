package authz

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
)

type fakeCache struct {
	data map[string]string
}

func newFakeCache() *fakeCache {
	return &fakeCache{data: make(map[string]string)}
}

func (f *fakeCache) Get(_ context.Context, key string) (string, error) {
	v, ok := f.data[key]
	if !ok {
		return "", errors.New("cache miss")
	}
	return v, nil
}

func (f *fakeCache) Set(_ context.Context, key string, value interface{}, _ time.Duration) error {
	f.data[key] = value.(string)
	return nil
}

func (f *fakeCache) Del(_ context.Context, keys ...string) error {
	for _, k := range keys {
		delete(f.data, k)
	}
	return nil
}

func TestCheckPrincipal_Active(t *testing.T) {
	cache := newFakeCache()
	checker := NewChecker(cache, func(_ context.Context, userID uuid.UUID) (UserStatus, error) {
		return StatusActive, nil
	})

	ctx := principal.WithPrincipal(context.Background(), principal.Principal{UserID: uuid.New().String()})
	err := checker.CheckPrincipal(ctx)
	require.NoError(t, err)
}

func TestCheckPrincipal_Missing(t *testing.T) {
	cache := newFakeCache()
	checker := NewChecker(cache, nil)

	err := checker.CheckPrincipal(context.Background())
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
}

func TestCheckPrincipal_Inactive(t *testing.T) {
	cache := newFakeCache()
	checker := NewChecker(cache, func(_ context.Context, userID uuid.UUID) (UserStatus, error) {
		return StatusInactive, nil
	})

	ctx := principal.WithPrincipal(context.Background(), principal.Principal{UserID: uuid.New().String()})
	err := checker.CheckPrincipal(ctx)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.PermissionDenied, st.Code())
}

func TestCheckPrincipal_NotFound(t *testing.T) {
	cache := newFakeCache()
	checker := NewChecker(cache, func(_ context.Context, userID uuid.UUID) (UserStatus, error) {
		return StatusNotFound, nil
	})

	ctx := principal.WithPrincipal(context.Background(), principal.Principal{UserID: uuid.New().String()})
	err := checker.CheckPrincipal(ctx)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
}

func TestCheckPrincipal_InvalidUUID(t *testing.T) {
	cache := newFakeCache()
	checker := NewChecker(cache, nil)

	ctx := principal.WithPrincipal(context.Background(), principal.Principal{UserID: "not-a-uuid"})
	err := checker.CheckPrincipal(ctx)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
}

func TestMustHaveRole_Success(t *testing.T) {
	cache := newFakeCache()
	checker := NewChecker(cache, nil)

	ctx := principal.WithPrincipal(context.Background(), principal.Principal{
		UserID: uuid.New().String(),
		Roles:  []string{"user", "admin"},
	})
	err := checker.MustHaveRole(ctx, "admin")
	require.NoError(t, err)
}

func TestMustHaveRole_MultipleRequired(t *testing.T) {
	cache := newFakeCache()
	checker := NewChecker(cache, nil)

	ctx := principal.WithPrincipal(context.Background(), principal.Principal{
		UserID: uuid.New().String(),
		Roles:  []string{"user"},
	})
	err := checker.MustHaveRole(ctx, "admin", "superuser")
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.PermissionDenied, st.Code())
}

func TestMustHaveRole_MissingPrincipal(t *testing.T) {
	cache := newFakeCache()
	checker := NewChecker(cache, nil)

	err := checker.MustHaveRole(context.Background(), "admin")
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
}

func TestCacheHit(t *testing.T) {
	cache := newFakeCache()
	lookupCalled := 0
	checker := NewChecker(cache, func(_ context.Context, userID uuid.UUID) (UserStatus, error) {
		lookupCalled++
		return StatusActive, nil
	})

	userID := uuid.New()
	// First call should hit the DB
	err := checker.CheckUser(context.Background(), userID)
	require.NoError(t, err)
	require.Equal(t, 1, lookupCalled)

	// Second call should hit the cache
	err = checker.CheckUser(context.Background(), userID)
	require.NoError(t, err)
	require.Equal(t, 1, lookupCalled)
}

func TestInvalidate(t *testing.T) {
	cache := newFakeCache()
	checker := NewChecker(cache, func(_ context.Context, userID uuid.UUID) (UserStatus, error) {
		return StatusActive, nil
	})

	userID := uuid.New()
	ctx := context.Background()

	// Warm cache
	_ = checker.CheckUser(ctx, userID)
	_, ok := cache.data[userStatusPrefix+userID.String()]
	require.True(t, ok)

	// Invalidate
	err := checker.Invalidate(ctx, userID)
	require.NoError(t, err)

	_, ok = cache.data[userStatusPrefix+userID.String()]
	require.False(t, ok)
}

func TestGRPCError(t *testing.T) {
	tests := []struct {
		status  UserStatus
		wantErr bool
		code    codes.Code
	}{
		{StatusActive, false, codes.OK},
		{StatusInactive, true, codes.PermissionDenied},
		{StatusNotFound, true, codes.Unauthenticated},
		{StatusUnknown, true, codes.Internal},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			err := GRPCError(tt.status)
			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			st, ok := status.FromError(err)
			require.True(t, ok)
			require.Equal(t, tt.code, st.Code())
		})
	}
}
