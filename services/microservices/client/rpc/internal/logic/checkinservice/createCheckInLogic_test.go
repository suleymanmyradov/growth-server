package checkinservicelogic

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

// noopTxRunner runs fn directly with a nil pgx.Tx — getTxRepo resolves to the
// mocked svcCtx.Repo whenever testTxRunner is set on the logic.
type noopTxRunner struct{}

func (noopTxRunner) Run(_ context.Context, _ string, fn func(pgx.Tx) error) error {
	return fn(nil)
}

// The fakes below embed the repository interfaces so uncalled methods panic
// instead of silently succeeding — only the methods under test are stubbed.

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
	habit  db.GetHabitRow
	err    error
	streak int32
}

func (f *fakeHabits) GetHabitByID(_ context.Context, _ uuid.UUID, _ string) (db.GetHabitRow, error) {
	return f.habit, f.err
}

func (f *fakeHabits) GetHabitStreak(_ context.Context, _, _ uuid.UUID, _ string) (int32, error) {
	return f.streak, nil
}

type fakeCheckIns struct {
	repository.ICheckIns
	existing    db.CheckIn
	existingErr error
	upserted    db.CheckIn
	upsertErr   error
	upsertCalls int
	lastParams  db.UpsertCheckInParams
}

func (f *fakeCheckIns) GetTodayCheckInByHabit(_ context.Context, _ uuid.UUID, _ string) (db.CheckIn, error) {
	return f.existing, f.existingErr
}

func (f *fakeCheckIns) UpsertCheckIn(_ context.Context, params db.UpsertCheckInParams) (db.CheckIn, error) {
	f.upsertCalls++
	f.lastParams = params
	return f.upserted, f.upsertErr
}

type fakeActivities struct {
	repository.IActivities
	calls int
}

func (f *fakeActivities) CreateActivityDeduped(_ context.Context, _ db.CreateActivityDedupedParams) error {
	f.calls++
	return nil
}

type fakeGoals struct {
	repository.IGoals
	linked []uuid.UUID
}

func (f *fakeGoals) ListGoalIDsByHabit(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
	return f.linked, nil
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

func newCheckInSvcCtx(prefs *fakeUserPrefs, habits *fakeHabits, checkIns *fakeCheckIns, activities *fakeActivities, goals *fakeGoals, outbox *fakeEventOutbox) *svc.ServiceContext {
	return &svc.ServiceContext{
		Repo: &repository.Repository{
			UserPreferences: prefs,
			Habits:          habits,
			CheckIns:        checkIns,
			Activities:      activities,
			Goals:           goals,
			EventOutbox:     outbox,
		},
	}
}

func checkInCtx(userID uuid.UUID) context.Context {
	return principal.WithPrincipal(context.Background(), principal.Principal{UserID: userID.String()})
}

func makeCheckInSvcCtx() (*svc.ServiceContext, *fakeCheckIns, *fakeActivities, *fakeEventOutbox, uuid.UUID, uuid.UUID) {
	userID := uuid.New()
	habitID := uuid.New()
	checkIns := &fakeCheckIns{existingErr: pgx.ErrNoRows}
	habits := &fakeHabits{
		habit: db.GetHabitRow{
			ID:     habitID,
			UserID: userID,
			Name:   "Meditate",
			Status: "active",
		},
		streak: 5,
	}
	activities := &fakeActivities{}
	outbox := &fakeEventOutbox{}
	prefs := &fakeUserPrefs{prefs: db.UserPreference{Timezone: "UTC"}}
	goals := &fakeGoals{}
	svcCtx := newCheckInSvcCtx(prefs, habits, checkIns, activities, goals, outbox)
	return svcCtx, checkIns, activities, outbox, userID, habitID
}

func TestCreateCheckIn_Success_EnqueuesEventAndActivity(t *testing.T) {
	svcCtx, checkIns, activities, outbox, userID, habitID := makeCheckInSvcCtx()
	checkInID := uuid.New()
	checkIns.upserted = db.CheckIn{
		ID:      checkInID,
		UserID:  userID,
		HabitID: habitID,
		Status:  "completed",
		Version: 1,
		LocalDate: pgtype.Date{
			Time:  time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC),
			Valid: true,
		},
	}

	l := NewCreateCheckInLogic(checkInCtx(userID), svcCtx)
	l.testTxRunner = noopTxRunner{}

	resp, err := l.CreateCheckIn(&client.CreateCheckInRequest{
		HabitId: habitID.String(),
		Status:  "completed",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, checkInID.String(), resp.CheckIn.Id)
	assert.Equal(t, 1, checkIns.upsertCalls)
	assert.Equal(t, 1, activities.calls, "first check-in must log one activity")
	require.Len(t, outbox.envs, 1, "check_in_created must land in the outbox")
	assert.Equal(t, string(events.TypeCheckInCreated), outbox.envs[0].EventType)
	assert.Equal(t, checkInEventID(checkInID, 1), outbox.envs[0].EventID, "event ID must be deterministic (check_in_id, version)")

	var payload events.CheckInCreated
	require.NoError(t, json.Unmarshal(outbox.envs[0].Payload, &payload))
	assert.Equal(t, userID.String(), payload.UserID)
	assert.Equal(t, habitID.String(), payload.HabitID)
	assert.Equal(t, "completed", payload.Status)
	assert.Equal(t, "2026-09-30", payload.LocalDate)
}

func TestCreateCheckIn_IdenticalRetry_NoActivityNoEvent(t *testing.T) {
	svcCtx, checkIns, activities, outbox, userID, habitID := makeCheckInSvcCtx()
	mood := "good"
	checkIns.existingErr = nil
	checkIns.existing = db.CheckIn{
		ID:      uuid.New(),
		UserID:  userID,
		HabitID: habitID,
		Status:  "completed",
		Mood:    &mood,
		Version: 3,
	}
	checkIns.upserted = checkIns.existing

	l := NewCreateCheckInLogic(checkInCtx(userID), svcCtx)
	l.testTxRunner = noopTxRunner{}

	_, err := l.CreateCheckIn(&client.CreateCheckInRequest{
		HabitId: habitID.String(),
		Status:  "completed",
		Mood:    "good",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, checkIns.upsertCalls, "upsert still runs (row touch)")
	assert.Equal(t, 0, activities.calls, "identical retry must not create a second activity")
	assert.Empty(t, outbox.envs, "identical retry must not enqueue a duplicate event")
}

func TestCreateCheckIn_StatusTransition_EmitsNewEventWithBumpedVersion(t *testing.T) {
	svcCtx, checkIns, activities, outbox, userID, habitID := makeCheckInSvcCtx()
	existingID := uuid.New()
	checkIns.existingErr = nil
	checkIns.existing = db.CheckIn{
		ID:      existingID,
		UserID:  userID,
		HabitID: habitID,
		Status:  "missed",
		Version: 2,
	}
	// Upsert returns the row at the post-transition version.
	checkIns.upserted = db.CheckIn{
		ID:      existingID,
		UserID:  userID,
		HabitID: habitID,
		Status:  "completed",
		Version: 3,
	}

	l := NewCreateCheckInLogic(checkInCtx(userID), svcCtx)
	l.testTxRunner = noopTxRunner{}

	_, err := l.CreateCheckIn(&client.CreateCheckInRequest{
		HabitId: habitID.String(),
		Status:  "completed",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, activities.calls, "status transition logs a new activity")
	require.Len(t, outbox.envs, 1)
	assert.Equal(t, checkInEventID(existingID, 3), outbox.envs[0].EventID, "transition event uses the post-bump version")
}

func TestCreateCheckIn_ValidationErrors(t *testing.T) {
	userID := uuid.New()
	svcCtx := newCheckInSvcCtx(nil, nil, nil, nil, nil, nil)
	l := NewCreateCheckInLogic(checkInCtx(userID), svcCtx)
	l.testTxRunner = noopTxRunner{}

	_, err := l.CreateCheckIn(&client.CreateCheckInRequest{Status: "completed"})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = l.CreateCheckIn(&client.CreateCheckInRequest{HabitId: uuid.New().String()})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = l.CreateCheckIn(&client.CreateCheckInRequest{HabitId: uuid.New().String(), Status: "bogus"})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = l.CreateCheckIn(&client.CreateCheckInRequest{HabitId: "not-a-uuid", Status: "completed"})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCreateCheckIn_Unauthenticated(t *testing.T) {
	svcCtx, _, _, _, _, habitID := makeCheckInSvcCtx()
	l := NewCreateCheckInLogic(context.Background(), svcCtx)
	l.testTxRunner = noopTxRunner{}

	_, err := l.CreateCheckIn(&client.CreateCheckInRequest{HabitId: habitID.String(), Status: "completed"})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestCreateCheckIn_HabitNotFound(t *testing.T) {
	svcCtx, checkIns, _, _, userID, habitID := makeCheckInSvcCtx()
	_ = checkIns
	habits := svcCtx.Repo.Habits.(*fakeHabits)
	habits.err = errors.New("not found")

	l := NewCreateCheckInLogic(checkInCtx(userID), svcCtx)
	l.testTxRunner = noopTxRunner{}

	_, err := l.CreateCheckIn(&client.CreateCheckInRequest{HabitId: habitID.String(), Status: "completed"})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestCreateCheckIn_OtherUsersHabit(t *testing.T) {
	svcCtx, _, _, _, _, habitID := makeCheckInSvcCtx()
	otherUser := uuid.New()
	svcCtx.Repo.Habits.(*fakeHabits).habit.UserID = otherUser

	l := NewCreateCheckInLogic(checkInCtx(uuid.New()), svcCtx)
	l.testTxRunner = noopTxRunner{}

	_, err := l.CreateCheckIn(&client.CreateCheckInRequest{HabitId: habitID.String(), Status: "completed"})
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestCreateCheckIn_OutboxFailureAbortsTx(t *testing.T) {
	svcCtx, checkIns, _, outbox, userID, habitID := makeCheckInSvcCtx()
	checkIns.upserted = db.CheckIn{ID: uuid.New(), UserID: userID, HabitID: habitID, Status: "completed", Version: 1}
	outbox.err = errors.New("outbox write failed")

	l := NewCreateCheckInLogic(checkInCtx(userID), svcCtx)
	l.testTxRunner = noopTxRunner{}

	_, err := l.CreateCheckIn(&client.CreateCheckInRequest{HabitId: habitID.String(), Status: "completed"})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err), "outbox failure must roll back the whole check-in tx")
}

func TestCheckInEventID_DeterministicPerVersion(t *testing.T) {
	id := uuid.New()
	assert.Equal(t, checkInEventID(id, 1), checkInEventID(id, 1))
	assert.NotEqual(t, checkInEventID(id, 1), checkInEventID(id, 2))
	assert.NotEqual(t, checkInEventID(id, 1), checkInEventID(uuid.New(), 1))
}
