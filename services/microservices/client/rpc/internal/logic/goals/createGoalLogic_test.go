package goalslogic

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

// fakeBilling embeds IBilling so uncalled methods panic instead of silently
// succeeding — tests only override EntitlementsOrFreeFallback.
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

func createGoalSvcCtx(b repository.IBilling, prefs repository.IUserPreferences) *svc.ServiceContext {
	return &svc.ServiceContext{
		Repo: &repository.Repository{Billing: b, UserPreferences: prefs},
	}
}

func createGoalCtx(userID uuid.UUID) context.Context {
	return principal.WithPrincipal(context.Background(), principal.Principal{UserID: userID.String()})
}

// A4: when billing cannot determine entitlements at all (subscription lookup
// failed AND the free-plan fallback failed), the request must be rejected —
// previously a billing error silently skipped limit enforcement.
func TestCreateGoal_BillingFailureFailsClosed(t *testing.T) {
	userID := uuid.New()
	b := &fakeBilling{err: errors.New("postgres down")}
	logic := NewCreateGoalLogic(createGoalCtx(userID), createGoalSvcCtx(b, nil))

	_, err := logic.CreateGoal(&client.CreateGoalRequest{
		Title:    "Run a marathon",
		Category: "fitness",
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "plan limits")
}

// A4: a user over the free-plan goal limit with completed onboarding is
// rejected with FailedPrecondition + PlanLimitDetail.
func TestCreateGoal_PlanLimitReached(t *testing.T) {
	userID := uuid.New()
	b := &fakeBilling{ent: &repository.EntitlementsResult{
		PlanCode:      "free",
		CanCreateGoal: false,
	}}
	prefs := &fakeUserPrefs{prefs: db.UserPreference{OnboardingCompleted: true}}
	logic := NewCreateGoalLogic(createGoalCtx(userID), createGoalSvcCtx(b, prefs))

	_, err := logic.CreateGoal(&client.CreateGoalRequest{
		Title:    "Another goal",
		Category: "fitness",
	})
	require.Error(t, err)
	st := status.Convert(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
	var foundDetail bool
	for _, d := range st.Details() {
		if pd, ok := d.(*client.PlanLimitDetail); ok {
			foundDetail = true
			assert.Equal(t, "active_goals", pd.Limit)
			assert.Equal(t, "goal_limit", pd.UpgradeTrigger)
		}
	}
	assert.True(t, foundDetail, "expected PlanLimitDetail in status details")
}
