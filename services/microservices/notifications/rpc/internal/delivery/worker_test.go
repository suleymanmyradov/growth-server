package delivery

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/unsubtoken"
)

func TestQuietHoursDeferral(t *testing.T) {
	loc := time.UTC

	mk := func(hour, min int) time.Time {
		return time.Date(2026, 10, 14, hour, min, 0, 0, loc)
	}

	t.Run("evening quiet hours defer to next morning 08:00", func(t *testing.T) {
		wake, quiet := quietHoursDeferral(mk(21, 0), loc)
		assert.True(t, quiet)
		assert.Equal(t, mk(8, 0).AddDate(0, 0, 1), wake)

		wake, quiet = quietHoursDeferral(mk(23, 59), loc)
		assert.True(t, quiet)
		assert.Equal(t, mk(8, 0).AddDate(0, 0, 1), wake)
	})

	t.Run("early morning quiet hours defer to same-day 08:00", func(t *testing.T) {
		wake, quiet := quietHoursDeferral(mk(0, 0), loc)
		assert.True(t, quiet)
		assert.Equal(t, mk(8, 0), wake)

		wake, quiet = quietHoursDeferral(mk(7, 59), loc)
		assert.True(t, quiet)
		assert.Equal(t, mk(8, 0), wake)
	})

	t.Run("daytime is not quiet", func(t *testing.T) {
		for _, h := range []int{8, 12, 17, 20} {
			_, quiet := quietHoursDeferral(mk(h, 0), loc)
			assert.False(t, quiet, "hour %d should not be quiet", h)
		}
	})

	t.Run("boundary 20:59 active, 08:00 active", func(t *testing.T) {
		_, quiet := quietHoursDeferral(mk(20, 59), loc)
		assert.False(t, quiet)
		_, quiet = quietHoursDeferral(mk(8, 0), loc)
		assert.False(t, quiet)
	})
}

func TestQuietHoursDeferral_TimezoneAware(t *testing.T) {
	// 2026-10-14 18:30 UTC is 14:30 in New York (EDT, UTC-4) — not quiet —
	// but 03:30 JST on Oct 15 — quiet.
	now := time.Date(2026, 10, 14, 18, 30, 0, 0, time.UTC)

	nyc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	_, quiet := quietHoursDeferral(now, nyc)
	assert.False(t, quiet, "14:30 in New York must not be quiet")

	tokyo, err := time.LoadLocation("Asia/Tokyo")
	require.NoError(t, err)
	wake, quiet := quietHoursDeferral(now, tokyo)
	assert.True(t, quiet, "03:30 in Tokyo must be quiet")
	assert.Equal(t, 8, wake.In(tokyo).Hour())
}

func TestPushBody_CoachFeedbackIsGeneric(t *testing.T) {
	// Lock-screen pushes for ai_feedback must never carry the generated coach
	// text — only a generic notice (or an explicit pushMessage override).
	n := db.GetNotificationRow{
		Type:    "ai_feedback",
		Message: "your deep personalized AI-generated paragraph with sensitive content",
	}
	assert.Equal(t, "Your coach has new feedback for you.", pushBody(n))

	n.Metadata = []byte(`{"pushMessage":"Coach reply ready"}`)
	assert.Equal(t, "Coach reply ready", pushBody(n))
}

func TestPushBody_OtherTypesUseMessage(t *testing.T) {
	n := db.GetNotificationRow{Type: "habit_reminder", Message: "Time to meditate"}
	assert.Equal(t, "Time to meditate", pushBody(n))
}

func TestPushMessageOverride_InvalidMetadata(t *testing.T) {
	assert.Equal(t, "", pushMessageOverride(nil))
	assert.Equal(t, "", pushMessageOverride([]byte(`{invalid`)))
	assert.Equal(t, "", pushMessageOverride([]byte(`{"other":1}`)))
}

func TestUnsubscribeHeaders_RFC8058(t *testing.T) {
	w := NewWorker(nil, nil, nil, "https://app.example.com/", "https://api.example.com/", "test-secret")
	userID := uuid.New()

	headers := w.unsubscribeHeaders(userID)
	require.Contains(t, headers, "List-Unsubscribe")
	assert.Equal(t, "List-Unsubscribe=One-Click", headers["List-Unsubscribe-Post"])

	// Header format is "<url>" and the URL must hit the API origin, not the app.
	raw := headers["List-Unsubscribe"]
	require.True(t, strings.HasPrefix(raw, "<") && strings.HasSuffix(raw, ">"), "List-Unsubscribe must be angle-bracketed: %q", raw)
	url := strings.TrimSuffix(strings.TrimPrefix(raw, "<"), ">")
	require.True(t, strings.HasPrefix(url, "https://api.example.com/api/v1/notifications/email-unsubscribe?token="), "unexpected URL: %s", url)

	// The embedded token must verify for this user with this secret.
	token := strings.TrimPrefix(url, "https://api.example.com/api/v1/notifications/email-unsubscribe?token=")
	got, err := unsubtoken.Verify("test-secret", token)
	require.NoError(t, err)
	assert.Equal(t, userID, got)
}

func TestUnsubscribeHeaders_DisabledWithoutConfig(t *testing.T) {
	w := NewWorker(nil, nil, nil, "https://app.example.com", "", "secret")
	assert.Nil(t, w.unsubscribeHeaders(uuid.New()))
	w = NewWorker(nil, nil, nil, "https://app.example.com", "https://api.example.com", "")
	assert.Nil(t, w.unsubscribeHeaders(uuid.New()))
}

func TestRenderEmail_HasUnsubscribeFooterLink(t *testing.T) {
	w := NewWorker(nil, nil, nil, "https://app.example.com", "https://api.example.com", "secret")
	html, err := w.renderEmail(db.GetNotificationRow{Title: "t", Message: "m"}, "Ada")
	require.NoError(t, err)
	assert.Contains(t, html, `href="https://app.example.com/me"`)
	assert.Contains(t, html, "Unsubscribe")
}

// Compile-time check that delivery rows still carry the fields the worker
// relies on (a schema refactor that drops them must fail here, not at runtime).
func TestDeliveryRowShape(t *testing.T) {
	var d db.NotificationDelivery
	d.ID = uuid.New()
	d.UserID = uuid.New()
	d.Channel = "push"
	d.AttemptCount = 0
	_ = pgtype.Timestamptz{}
}
