package settingslogic

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

// noopTxRunner runs fn directly with a nil pgx.Tx — runInTx resolves to the
// mocked svcCtx.Repo whenever a testTxRunner override is set on the logic.
type noopTxRunner struct{}

func (noopTxRunner) Run(_ context.Context, _ string, fn func(pgx.Tx) error) error {
	return fn(nil)
}

// Fakes embed the repository interfaces so uncalled methods panic instead of
// silently succeeding.

type fakeUserPrefs struct {
	repository.IUserPreferences
	prefs             db.UserPreference
	getErr            error
	onboardingCalls   int
	lastCheckInTime   pgtype.Time
	lastOnboarding    bool
	updatePrefsCalls  int
	lastTheme         string
	lastLanguage      string
	lastTimezone      string
	updatePrefsReturn db.UserPreference
	updatePrefsErr    error
}

func (f *fakeUserPrefs) GetUserPreferences(_ context.Context, _ uuid.UUID) (db.UserPreference, error) {
	return f.prefs, f.getErr
}

func (f *fakeUserPrefs) UpdateOnboardingCompleted(_ context.Context, _ uuid.UUID, checkInTime pgtype.Time, onboardingCompleted bool) (db.UserPreference, error) {
	f.onboardingCalls++
	f.lastCheckInTime = checkInTime
	f.lastOnboarding = onboardingCompleted
	return db.UserPreference{}, nil
}

func (f *fakeUserPrefs) UpdateUserPreferences(_ context.Context, _ uuid.UUID, theme, language, timezone string) (db.UserPreference, error) {
	f.updatePrefsCalls++
	f.lastTheme = theme
	f.lastLanguage = language
	f.lastTimezone = timezone
	return f.updatePrefsReturn, f.updatePrefsErr
}

type fakeCoachingProfiles struct {
	repository.ICoachingProfiles
	calls              int
	lastAccountability string
	updatePrefsErr     error
}

func (f *fakeCoachingProfiles) UpdateCoachingProfilePreferences(_ context.Context, _ uuid.UUID, accountabilityStyle, _, _ string) (db.UpdateCoachingProfilePreferencesRow, error) {
	f.calls++
	f.lastAccountability = accountabilityStyle
	return db.UpdateCoachingProfilePreferencesRow{}, f.updatePrefsErr
}

type fakeEventOutbox struct {
	repository.IEventOutbox
	envs []events.Envelope
	err  error
}

func (f *fakeEventOutbox) Enqueue(_ context.Context, env events.Envelope) error {
	f.envs = append(f.envs, env)
	return f.err
}

func settingsSvcCtx(prefs repository.IUserPreferences, profiles repository.ICoachingProfiles, outbox repository.IEventOutbox) *svc.ServiceContext {
	return &svc.ServiceContext{
		Repo: &repository.Repository{
			UserPreferences:  prefs,
			CoachingProfiles: profiles,
			EventOutbox:      outbox,
		},
	}
}

func settingsCtx(userID uuid.UUID) context.Context {
	return principal.WithPrincipal(context.Background(), principal.Principal{UserID: userID.String()})
}

func TestUpdateSettings_OnboardingEnqueuesUserOnboardedAndSettingsChanged(t *testing.T) {
	userID := uuid.New()
	prefs := &fakeUserPrefs{prefs: db.UserPreference{Theme: "system", Language: "en", Timezone: "UTC"}}
	profiles := &fakeCoachingProfiles{}
	outbox := &fakeEventOutbox{}

	l := NewUpdateSettingsLogic(settingsCtx(userID), settingsSvcCtx(prefs, profiles, outbox))
	l.testTxRunner = noopTxRunner{}

	resp, err := l.UpdateSettings(&client.UpdateSettingsRequest{
		Settings: &client.UserSettings{
			OnboardingCompleted: true,
			CheckInTime:         "08:30",
			AccountabilityStyle: "firm",
			Timezone:            "Europe/Berlin",
		},
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)

	assert.Equal(t, 1, prefs.onboardingCalls)
	assert.Equal(t, int64((8*3600+30*60)*1_000_000), prefs.lastCheckInTime.Microseconds)
	assert.True(t, prefs.lastOnboarding)
	assert.Equal(t, 1, profiles.calls)
	assert.Equal(t, "firm", profiles.lastAccountability)

	// Expect: user_onboarded + settings_changed + coaching_profile_changed.
	require.Len(t, outbox.envs, 3)
	types := []string{outbox.envs[0].EventType, outbox.envs[1].EventType, outbox.envs[2].EventType}
	assert.ElementsMatch(t, []string{
		string(events.TypeUserOnboarded),
		string(events.TypeSettingsChanged),
		string(events.TypeCoachingProfileChanged),
	}, types)

	for _, env := range outbox.envs {
		if env.EventType == string(events.TypeSettingsChanged) {
			var p events.SettingsChanged
			require.NoError(t, json.Unmarshal(env.Payload, &p))
			assert.Equal(t, "Europe/Berlin", p.Timezone)
			assert.Equal(t, "08:30", p.CheckInTime)
		}
	}
}

func TestUpdateSettings_PartialUpdatePreservesPersistedValues(t *testing.T) {
	userID := uuid.New()
	prefs := &fakeUserPrefs{prefs: db.UserPreference{
		Theme:    "dark",
		Language: "en",
		Timezone: "America/New_York",
	}}
	outbox := &fakeEventOutbox{}

	l := NewUpdateSettingsLogic(settingsCtx(userID), settingsSvcCtx(prefs, nil, outbox))
	l.testTxRunner = noopTxRunner{}

	// Only the theme is updated — language/timezone must be preserved from the
	// stored row, not reset to defaults.
	resp, err := l.UpdateSettings(&client.UpdateSettingsRequest{
		Settings: &client.UserSettings{Theme: "light"},
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, 1, prefs.updatePrefsCalls)
	assert.Equal(t, "light", prefs.lastTheme)
	assert.Equal(t, "en", prefs.lastLanguage)
	assert.Equal(t, "America/New_York", prefs.lastTimezone)
	assert.Empty(t, outbox.envs, "theme-only change publishes no domain events")
}

func TestUpdateSettings_InvalidTimezoneRejected(t *testing.T) {
	userID := uuid.New()
	prefs := &fakeUserPrefs{}
	outbox := &fakeEventOutbox{}

	l := NewUpdateSettingsLogic(settingsCtx(userID), settingsSvcCtx(prefs, nil, outbox))
	l.testTxRunner = noopTxRunner{}

	_, err := l.UpdateSettings(&client.UpdateSettingsRequest{
		Settings: &client.UserSettings{Timezone: "Not/AZone"},
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Equal(t, 0, prefs.updatePrefsCalls)
	assert.Empty(t, outbox.envs)
}

func TestUpdateSettings_InvalidCheckInTimeRejected(t *testing.T) {
	userID := uuid.New()
	prefs := &fakeUserPrefs{}

	l := NewUpdateSettingsLogic(settingsCtx(userID), settingsSvcCtx(prefs, nil, &fakeEventOutbox{}))
	l.testTxRunner = noopTxRunner{}

	_, err := l.UpdateSettings(&client.UpdateSettingsRequest{
		Settings: &client.UserSettings{CheckInTime: "25:99"},
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Equal(t, 0, prefs.onboardingCalls)
}

func TestUpdateSettings_Unauthenticated(t *testing.T) {
	l := NewUpdateSettingsLogic(context.Background(), settingsSvcCtx(nil, nil, nil))
	_, err := l.UpdateSettings(&client.UpdateSettingsRequest{Settings: &client.UserSettings{}})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestUpdateSettings_OutboxFailureAbortsTx(t *testing.T) {
	userID := uuid.New()
	prefs := &fakeUserPrefs{prefs: db.UserPreference{Theme: "system", Language: "en", Timezone: "UTC"}}
	outbox := &fakeEventOutbox{err: errors.New("outbox write failed")}

	l := NewUpdateSettingsLogic(settingsCtx(userID), settingsSvcCtx(prefs, nil, outbox))
	l.testTxRunner = noopTxRunner{}

	_, err := l.UpdateSettings(&client.UpdateSettingsRequest{
		Settings: &client.UserSettings{OnboardingCompleted: true},
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err), "outbox failure must roll back the settings tx")
}

func TestFormatCheckInTime(t *testing.T) {
	assert.Equal(t, "", formatCheckInTime(pgtype.Time{}))
	assert.Equal(t, "08:30", formatCheckInTime(pgtype.Time{
		Microseconds: int64((8*3600 + 30*60) * 1_000_000), Valid: true,
	}))
	assert.Equal(t, "23:59", formatCheckInTime(pgtype.Time{
		Microseconds: int64((23*3600 + 59*60) * 1_000_000), Valid: true,
	}))
	_ = time.Now // keep time import used if helpers change
}
