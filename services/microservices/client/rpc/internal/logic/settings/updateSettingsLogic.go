package settingslogic

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/pkg/validator"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// pgxTxRunner is satisfied by *postgres.PgxTxRunner; tests inject a no-op
// runner that calls fn(nil) so repository calls land on the mock svcCtx.Repo.
type pgxTxRunner interface {
	Run(ctx context.Context, userID string, fn func(pgx.Tx) error) error
}

// runInTx runs fn inside svcCtx.RunInTx in production; when override is set
// (tests only), fn runs directly against svcCtx.Repo without a real
// transaction — mirroring the billingservice testTxRunner seam.
func runInTx(svcCtx *svc.ServiceContext, override pgxTxRunner, ctx context.Context, userID string, fn func(*repository.Repository) error) error {
	if override != nil {
		return override.Run(ctx, userID, func(pgx.Tx) error {
			return fn(svcCtx.Repo)
		})
	}
	return svcCtx.RunInTx(ctx, userID, fn)
}

type UpdateSettingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
	// testTxRunner is only set in tests to inject a no-op transaction runner.
	testTxRunner pgxTxRunner
}

func NewUpdateSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSettingsLogic {
	return &UpdateSettingsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateSettingsLogic) UpdateSettings(in *client.UpdateSettingsRequest) (*client.UpdateSettingsResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "UpdateSettingsLogic.UpdateSettings")
	defer span.End()

	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, status.Error(codes.Internal, "invalid user id")
	}

	if l.svcCtx.Authz != nil {
		if err := l.svcCtx.Authz.CheckPrincipal(ctx); err != nil {
			return nil, err
		}
	}

	// Validate the timezone before any writes: an invalid IANA name would
	// poison every `AT TIME ZONE` habit query and turn them into 500s.
	if in.Settings != nil && in.Settings.Timezone != "" && !validator.IsValidTimezone(in.Settings.Timezone) {
		return nil, status.Error(codes.InvalidArgument, "invalid timezone")
	}

	// All settings writes + their events go through one transaction so the
	// outbox rows commit atomically with the preference changes (P1).
	err = runInTx(l.svcCtx, l.testTxRunner, ctx, userID.String(), func(txRepo *repository.Repository) error {
		// Handle onboarding settings update (check-in time, onboarding flag →
		// user_preferences; accountability style → coaching_profiles).
		var checkInTime pgtype.Time
		if in.Settings != nil && (in.Settings.AccountabilityStyle != "" || in.Settings.CheckInTime != "" || in.Settings.OnboardingCompleted) {
			if in.Settings.CheckInTime != "" {
				t, err := time.Parse("15:04", in.Settings.CheckInTime)
				if err != nil {
					return status.Error(codes.InvalidArgument, "invalid checkInTime format, expected HH:MM")
				}
				checkInTime = pgtype.Time{Microseconds: (int64(t.Hour())*3600 + int64(t.Minute())*60) * 1_000_000, Valid: true}
			}
			if _, err := txRepo.UserPreferences.UpdateOnboardingCompleted(ctx, userID, checkInTime, in.Settings.OnboardingCompleted); err != nil {
				return fmt.Errorf("update onboarding settings: %w", err)
			}

			// Update accountability style in coaching_profiles.
			if in.Settings.AccountabilityStyle != "" {
				if _, err := txRepo.CoachingProfiles.UpdateCoachingProfilePreferences(ctx, userID, in.Settings.AccountabilityStyle, "", ""); err != nil {
					return fmt.Errorf("update coaching profile: %w", err)
				}
			}
		}

		// General settings update (theme, language, timezone → user_preferences).
		if in.Settings != nil && (in.Settings.Theme != "" || in.Settings.Language != "" || in.Settings.Timezone != "") {
			// Fetch current preferences to preserve fields not being updated.
			// No row yet means the schema defaults are in effect (the upsert
			// below creates the row).
			theme, language, timezone := "system", "en", "UTC"
			current, err := txRepo.UserPreferences.GetUserPreferences(ctx, userID)
			if err == nil {
				theme, language, timezone = current.Theme, current.Language, current.Timezone
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("fetch user preferences: %w", err)
			}
			if in.Settings.Theme != "" {
				theme = in.Settings.Theme
			}
			if in.Settings.Language != "" {
				language = in.Settings.Language
			}
			if in.Settings.Timezone != "" {
				timezone = in.Settings.Timezone
			}
			if _, err := txRepo.UserPreferences.UpdateUserPreferences(ctx, userID, theme, language, timezone); err != nil {
				return fmt.Errorf("update user preferences: %w", err)
			}
		}

		// Enqueue domain events into the outbox inside this transaction.
		if in.Settings != nil && in.Settings.OnboardingCompleted {
			env, err := events.NewEnvelope(events.TypeUserOnboarded, events.UserOnboarded{
				UserID: userID.String(),
			})
			if err != nil {
				return fmt.Errorf("build user_onboarded envelope: %w", err)
			}
			if err := txRepo.EventOutbox.Enqueue(ctx, env); err != nil {
				return err
			}
		}

		if in.Settings != nil && (in.Settings.Timezone != "" || in.Settings.CheckInTime != "") {
			// Publish the merged persisted values, not just the fields present
			// in this request — the notifications consumer upserts whatever the
			// event carries, and a partial payload would reset the other column.
			mergedTimezone := in.Settings.Timezone
			mergedCheckIn := in.Settings.CheckInTime
			if cur, err := txRepo.UserPreferences.GetUserPreferences(ctx, userID); err == nil {
				if mergedTimezone == "" {
					mergedTimezone = cur.Timezone
				}
				if mergedCheckIn == "" && cur.CheckInTime.Valid {
					mergedCheckIn = formatCheckInTime(cur.CheckInTime)
				}
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("fetch user preferences for settings event: %w", err)
			}
			env, err := events.NewEnvelope(events.TypeSettingsChanged, events.SettingsChanged{
				UserID:      userID.String(),
				Timezone:    mergedTimezone,
				CheckInTime: mergedCheckIn,
			})
			if err != nil {
				return fmt.Errorf("build settings_changed envelope: %w", err)
			}
			if err := txRepo.EventOutbox.Enqueue(ctx, env); err != nil {
				return err
			}
		}

		// Sync the coaching profile read model owned by ai-coach-consumer (P2)
		// whenever accountability style is written here.
		if in.Settings != nil && in.Settings.AccountabilityStyle != "" {
			env, err := events.NewEnvelope(events.TypeCoachingProfileChanged, events.CoachingProfileChanged{
				UserID:              userID.String(),
				AccountabilityStyle: in.Settings.AccountabilityStyle,
			})
			if err != nil {
				return fmt.Errorf("build coaching_profile_changed envelope: %w", err)
			}
			if err := txRepo.EventOutbox.Enqueue(ctx, env); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() != codes.Unknown {
			return nil, st.Err()
		}
		l.Errorf("Failed to update settings: %v", err)
		return nil, status.Error(codes.Internal, "failed to update settings")
	}

	return &client.UpdateSettingsResponse{
		Success: true,
	}, nil
}

// formatCheckInTime renders a pgtype.Time as "HH:MM" for the SettingsChanged
// event payload (empty string when unset).
func formatCheckInTime(t pgtype.Time) string {
	if !t.Valid {
		return ""
	}
	totalMin := t.Microseconds / 60_000_000
	return fmt.Sprintf("%02d:%02d", totalMin/60, totalMin%60)
}
