package commonlogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
)

// IsOnboardingComplete reports whether the user has finished onboarding.
//
// A user with no user_preferences row (pgx.ErrNoRows) is treated as not
// onboarded. Any other DB error is also treated as "not complete" so that a
// transient failure never blocks the initial onboarding flow — the worst case
// is that a limit check is skipped, which is safe for a user who genuinely
// hasn't onboarded yet.
func IsOnboardingComplete(ctx context.Context, svcCtx *svc.ServiceContext, userID uuid.UUID) bool {
	prefs, err := svcCtx.Repo.UserPreferences.GetUserPreferences(ctx, userID)
	if err != nil {
		// Unexpected error (not ErrNoRows) — log nothing here (caller can log), fail open.
		return false
	}
	return prefs.OnboardingCompleted
}
