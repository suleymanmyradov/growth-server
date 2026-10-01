package habitslogic

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

type fakeBilling struct {
	repository.IBilling
	ent *repository.EntitlementsResult
	err error
}

func (f *fakeBilling) EntitlementsOrFreeFallback(_ context.Context, _ uuid.UUID) (*repository.EntitlementsResult, error) {
	return f.ent, f.err
}

type fakeUserPrefs struct {
	repository.IUserPreferences
	prefs db.UserPreference
	err   error
}

func (f *fakeUserPrefs) GetUserPreferences(_ context.Context, _ uuid.UUID) (db.UserPreference, error) {
	return f.prefs, f.err
}

type fakeHabits struct {
	repository.IHabits
	habit       db.GetHabitRow
	getErr      error
	created     db.GetHabitRow
	createErr   error
	createCalls int
	updated     db.GetHabitRow
	updateErr   error
	lastUpdate  db.UpdateHabitParams
	deleted     bool
	deleteErr   error
	streak      int32
}

func (f *fakeHabits) GetHabitByID(_ context.Context, _ uuid.UUID, _ string) (db.GetHabitRow, error) {
	return f.habit, f.getErr
}

func (f *fakeHabits) CreateHabit(_ context.Context, name string, _ *string, category string, userID uuid.UUID) (db.GetHabitRow, error) {
	f.createCalls++
	if f.createErr != nil {
		return db.GetHabitRow{}, f.createErr
	}
	row := f.created
	row.ID = uuid.New()
	row.UserID = userID
	row.Name = name
	row.Category = category
	return row, nil
}

func (f *fakeHabits) UpdateHabit(_ context.Context, params db.UpdateHabitParams) (db.GetHabitRow, error) {
	f.lastUpdate = params
	return f.updated, f.updateErr
}

func (f *fakeHabits) DeleteHabit(_ context.Context, _ uuid.UUID) error {
	f.deleted = true
	return f.deleteErr
}

func (f *fakeHabits) GetHabitStreak(_ context.Context, _, _ uuid.UUID, _ string) (int32, error) {
	return f.streak, nil
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

func habitSvcCtx(billing repository.IBilling, prefs repository.IUserPreferences, habits repository.IHabits, outbox repository.IEventOutbox) *svc.ServiceContext {
	return &svc.ServiceContext{
		Repo: &repository.Repository{
			Billing:         billing,
			UserPreferences: prefs,
			Habits:          habits,
			EventOutbox:     outbox,
		},
	}
}

func habitCtx(userID uuid.UUID) context.Context {
	return principal.WithPrincipal(context.Background(), principal.Principal{UserID: userID.String()})
}

func TestCreateHabit_Success_EnqueuesHabitCreatedEvent(t *testing.T) {
	userID := uuid.New()
	billing := &fakeBilling{ent: &repository.EntitlementsResult{CanCreateHabit: true}}
	habits := &fakeHabits{}
	outbox := &fakeEventOutbox{}
	prefs := &fakeUserPrefs{prefs: db.UserPreference{Timezone: "UTC", OnboardingCompleted: true}}

	l := NewCreateHabitLogic(habitCtx(userID), habitSvcCtx(billing, prefs, habits, outbox))
	l.testTxRunner = noopTxRunner{}

	resp, err := l.CreateHabit(&client.CreateHabitRequest{
		Name:     "Meditate",
		Category: "mindfulness",
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Habit)
	assert.Equal(t, "Meditate", resp.Habit.Name)
	assert.Equal(t, 1, habits.createCalls)
	require.Len(t, outbox.envs, 1, "habit_created must land in the event outbox")
	assert.Equal(t, string(events.TypeHabitCreated), outbox.envs[0].EventType)
}

func TestCreateHabit_PlanLimitReached(t *testing.T) {
	userID := uuid.New()
	billing := &fakeBilling{ent: &repository.EntitlementsResult{PlanCode: "free", CanCreateHabit: false}}
	prefs := &fakeUserPrefs{prefs: db.UserPreference{OnboardingCompleted: true}}

	l := NewCreateHabitLogic(habitCtx(userID), habitSvcCtx(billing, prefs, &fakeHabits{}, nil))
	l.testTxRunner = noopTxRunner{}

	_, err := l.CreateHabit(&client.CreateHabitRequest{Name: "Another", Category: "x"})
	require.Error(t, err)
	st := status.Convert(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
	var foundDetail bool
	for _, d := range st.Details() {
		if pd, ok := d.(*client.PlanLimitDetail); ok {
			foundDetail = true
			assert.Equal(t, "habit_limit", pd.UpgradeTrigger)
		}
	}
	assert.True(t, foundDetail, "expected PlanLimitDetail in status details")
}

func TestCreateHabit_OnboardingBypassesPlanLimit(t *testing.T) {
	userID := uuid.New()
	billing := &fakeBilling{ent: &repository.EntitlementsResult{PlanCode: "free", CanCreateHabit: false}}
	prefs := &fakeUserPrefs{prefs: db.UserPreference{OnboardingCompleted: false}}
	habits := &fakeHabits{}
	outbox := &fakeEventOutbox{}

	l := NewCreateHabitLogic(habitCtx(userID), habitSvcCtx(billing, prefs, habits, outbox))
	l.testTxRunner = noopTxRunner{}

	resp, err := l.CreateHabit(&client.CreateHabitRequest{Name: "First habit", Category: "x"})
	require.NoError(t, err, "onboarding users must be able to create initial habits")
	require.NotNil(t, resp.Habit)
	assert.Equal(t, 1, habits.createCalls)
}

func TestCreateHabit_BillingFailureFailsClosed(t *testing.T) {
	userID := uuid.New()
	billing := &fakeBilling{err: errors.New("postgres down")}

	l := NewCreateHabitLogic(habitCtx(userID), habitSvcCtx(billing, nil, &fakeHabits{}, nil))
	l.testTxRunner = noopTxRunner{}

	_, err := l.CreateHabit(&client.CreateHabitRequest{Name: "x", Category: "x"})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestCreateHabit_Unauthenticated(t *testing.T) {
	l := NewCreateHabitLogic(context.Background(), habitSvcCtx(nil, nil, nil, nil))
	_, err := l.CreateHabit(&client.CreateHabitRequest{Name: "x", Category: "x"})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestUpdateHabit_BackfillsOmittedFields(t *testing.T) {
	userID := uuid.New()
	habitID := uuid.New()
	existingDesc := "morning session"
	habits := &fakeHabits{
		habit: db.GetHabitRow{
			ID:          habitID,
			UserID:      userID,
			Name:        "Meditate",
			Description: &existingDesc,
			Category:    "mindfulness",
		},
		updated: db.GetHabitRow{ID: habitID, UserID: userID, Name: "Meditate daily"},
	}
	prefs := &fakeUserPrefs{prefs: db.UserPreference{Timezone: "UTC"}}

	l := NewUpdateHabitLogic(habitCtx(userID), habitSvcCtx(nil, prefs, habits, nil))

	resp, err := l.UpdateHabit(&client.UpdateHabitRequest{
		HabitId: habitID.String(),
		Name:    "Meditate daily",
		// Description and Category omitted — must be backfilled, not blanked.
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Habit)
	assert.Equal(t, "Meditate daily", habits.lastUpdate.Name)
	require.NotNil(t, habits.lastUpdate.Description)
	assert.Equal(t, "morning session", *habits.lastUpdate.Description)
	assert.Equal(t, "mindfulness", habits.lastUpdate.Slug)
}

func TestUpdateHabit_OtherUsersHabitDenied(t *testing.T) {
	habits := &fakeHabits{habit: db.GetHabitRow{ID: uuid.New(), UserID: uuid.New()}}
	prefs := &fakeUserPrefs{prefs: db.UserPreference{Timezone: "UTC"}}

	l := NewUpdateHabitLogic(habitCtx(uuid.New()), habitSvcCtx(nil, prefs, habits, nil))

	_, err := l.UpdateHabit(&client.UpdateHabitRequest{HabitId: uuid.New().String(), Name: "x"})
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestDeleteHabit_Success_EnqueuesHabitDeletedEvent(t *testing.T) {
	userID := uuid.New()
	habitID := uuid.New()
	habits := &fakeHabits{habit: db.GetHabitRow{ID: habitID, UserID: userID}}
	outbox := &fakeEventOutbox{}
	prefs := &fakeUserPrefs{prefs: db.UserPreference{Timezone: "UTC"}}

	l := NewDeleteHabitLogic(habitCtx(userID), habitSvcCtx(nil, prefs, habits, outbox))
	l.testTxRunner = noopTxRunner{}

	resp, err := l.DeleteHabit(&client.DeleteHabitRequest{HabitId: habitID.String()})
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.True(t, habits.deleted)
	require.Len(t, outbox.envs, 1)
	assert.Equal(t, string(events.TypeHabitDeleted), outbox.envs[0].EventType)
}

func TestDeleteHabit_OtherUsersHabitDenied(t *testing.T) {
	habitID := uuid.New()
	habits := &fakeHabits{habit: db.GetHabitRow{ID: habitID, UserID: uuid.New()}}
	prefs := &fakeUserPrefs{prefs: db.UserPreference{Timezone: "UTC"}}
	outbox := &fakeEventOutbox{}

	l := NewDeleteHabitLogic(habitCtx(uuid.New()), habitSvcCtx(nil, prefs, habits, outbox))
	l.testTxRunner = noopTxRunner{}

	_, err := l.DeleteHabit(&client.DeleteHabitRequest{HabitId: habitID.String()})
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.False(t, habits.deleted)
	assert.Empty(t, outbox.envs)
}
