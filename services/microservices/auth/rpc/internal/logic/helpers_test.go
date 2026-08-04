package logic

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
)

// ============================================
// toNullString
// ============================================

func TestToNullString(t *testing.T) {
	t.Run("returns nil for empty string", func(t *testing.T) {
		result := toNullString("")
		assert.Nil(t, result)
	})

	t.Run("returns nil for whitespace-only string", func(t *testing.T) {
		result := toNullString("   ")
		assert.Nil(t, result)
	})

	t.Run("returns pointer for non-empty string", func(t *testing.T) {
		result := toNullString("hello")
		require.NotNil(t, result)
		assert.Equal(t, "hello", *result)
	})

	t.Run("returns pointer for string with leading/trailing spaces", func(t *testing.T) {
		// toNullString trims and checks; "  hello  " has non-whitespace content
		result := toNullString("  hello  ")
		require.NotNil(t, result)
		assert.Equal(t, "  hello  ", *result)
	})
}

// ============================================
// nonNilStrings
// ============================================

func TestNonNilStrings(t *testing.T) {
	t.Run("returns empty slice for nil input", func(t *testing.T) {
		result := nonNilStrings(nil)
		assert.Equal(t, []string{}, result)
	})

	t.Run("returns empty slice for empty slice input", func(t *testing.T) {
		result := nonNilStrings([]string{})
		assert.Equal(t, []string{}, result)
	})

	t.Run("returns the slice as-is for non-empty input", func(t *testing.T) {
		input := []string{"a", "b", "c"}
		result := nonNilStrings(input)
		assert.Equal(t, input, result)
	})
}

// ============================================
// formatTime
// ============================================

func TestFormatTime(t *testing.T) {
	t.Run("formats a UTC timestamp with Z suffix", func(t *testing.T) {
		ts := pgtype.Timestamptz{Time: time.Date(2026, 1, 15, 12, 30, 45, 0, time.UTC), Valid: true}
		result := formatTime(ts)
		assert.Equal(t, "2026-01-15T12:30:45Z", result)
	})

	t.Run("formats a non-UTC timestamp with offset", func(t *testing.T) {
		loc, _ := time.LoadLocation("America/New_York")
		ts := pgtype.Timestamptz{Time: time.Date(2026, 1, 15, 7, 30, 45, 0, loc), Valid: true}
		result := formatTime(ts)
		// EST is UTC-5
		assert.Equal(t, "2026-01-15T07:30:45-05:00", result)
	})
}

// ============================================
// toPbUser
// ============================================

func TestToPbUser(t *testing.T) {
	userID := uuid.New()
	now := time.Now().UTC()

	t.Run("maps all fields correctly with non-nil optional fields", func(t *testing.T) {
		bio := "I love habits"
		location := "NYC"
		website := "https://example.com"
		avatar := "https://cdn.example.com/avatar.png"
		user := db.User{
			ID:            userID,
			Username:      "janedoe",
			Email:         "jane@example.com",
			PasswordHash:  strPtr("hashed"),
			FullName:      "Jane Doe",
			Bio:           &bio,
			Location:      &location,
			Website:       &website,
			Interests:     []string{"reading", "running"},
			AvatarUrl:     &avatar,
			CreatedAt:     pgtype.Timestamptz{Time: now, Valid: true},
			UpdatedAt:     pgtype.Timestamptz{Time: now, Valid: true},
			EmailVerified: true,
		}

		result := toPbUser(user)

		assert.Equal(t, userID.String(), result.Id)
		assert.Equal(t, "janedoe", result.Username)
		assert.Equal(t, "jane@example.com", result.Email)
		assert.Equal(t, "Jane Doe", result.FullName)
		assert.Equal(t, "I love habits", result.Bio)
		assert.Equal(t, "NYC", result.Location)
		assert.Equal(t, "https://example.com", result.Website)
		assert.Equal(t, []string{"reading", "running"}, result.Interests)
		assert.Equal(t, "https://cdn.example.com/avatar.png", result.AvatarUrl)
		assert.Equal(t, "jane@example.com", result.Email)
		assert.True(t, result.EmailVerified)
	})

	t.Run("defaults nil optional fields to empty strings", func(t *testing.T) {
		user := db.User{
			ID:            userID,
			Username:      "janedoe",
			Email:         "jane@example.com",
			FullName:      "Jane Doe",
			Bio:           nil,
			Location:      nil,
			Website:       nil,
			AvatarUrl:     nil,
			CreatedAt:     pgtype.Timestamptz{Time: now, Valid: true},
			UpdatedAt:     pgtype.Timestamptz{Time: now, Valid: true},
			EmailVerified: false,
		}

		result := toPbUser(user)

		assert.Equal(t, "", result.Bio)
		assert.Equal(t, "", result.Location)
		assert.Equal(t, "", result.Website)
		assert.Equal(t, "", result.AvatarUrl)
		assert.False(t, result.EmailVerified)
	})

	t.Run("converts nil interests to empty slice (not null in JSON)", func(t *testing.T) {
		user := db.User{
			ID:        userID,
			Interests: nil,
		}

		result := toPbUser(user)

		assert.Equal(t, []string{}, result.Interests)
	})

	t.Run("preserves empty interests slice as empty slice", func(t *testing.T) {
		user := db.User{
			ID:        userID,
			Interests: []string{},
		}

		result := toPbUser(user)

		assert.Equal(t, []string{}, result.Interests)
	})
}

// ============================================
// generateRandomToken
// ============================================

func TestGenerateRandomToken(t *testing.T) {
	t.Run("returns a hex-encoded string of the expected length", func(t *testing.T) {
		// 32 bytes → 64 hex chars
		token := generateRandomToken(32)
		assert.Len(t, token, 64)
	})

	t.Run("returns different tokens on subsequent calls", func(t *testing.T) {
		token1 := generateRandomToken(32)
		token2 := generateRandomToken(32)
		assert.NotEqual(t, token1, token2)
	})

	t.Run("produces valid hex", func(t *testing.T) {
		token := generateRandomToken(16)
		// Should only contain hex characters
		for _, c := range token {
			assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
				"unexpected non-hex character: %c", c)
		}
	})

	t.Run("handles length 0", func(t *testing.T) {
		token := generateRandomToken(0)
		assert.Equal(t, "", token)
	})
}

// ============================================
// helpers
// ============================================

func strPtr(s string) *string {
	return &s
}
