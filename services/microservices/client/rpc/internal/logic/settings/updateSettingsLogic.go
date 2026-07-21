package settingslogic

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdateSettingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
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

	// Handle onboarding settings update (check-in time, onboarding flag → user_preferences;
	// accountability style → coaching_profiles).
	if in.Settings != nil && (in.Settings.AccountabilityStyle != "" || in.Settings.CheckInTime != "" || in.Settings.OnboardingCompleted) {
		var checkInTime pgtype.Time
		if in.Settings.CheckInTime != "" {
			if t, err := time.Parse("15:04", in.Settings.CheckInTime); err == nil {
				checkInTime = pgtype.Time{Microseconds: (int64(t.Hour())*3600 + int64(t.Minute())*60) * 1_000_000, Valid: true}
			}
		}
		_, err = l.svcCtx.Repo.UserPreferences.UpdateOnboardingCompleted(ctx, userID, checkInTime, in.Settings.OnboardingCompleted)
		if err != nil {
			l.Errorf("Failed to update onboarding settings: %v", err)
			return nil, status.Error(codes.Internal, "failed to update onboarding settings")
		}

		// Update accountability style in coaching_profiles.
		if in.Settings.AccountabilityStyle != "" {
			_, err = l.svcCtx.Repo.CoachingProfiles.UpdateCoachingProfilePreferences(ctx, userID, in.Settings.AccountabilityStyle, "", "")
			if err != nil {
				l.Errorf("Failed to update coaching profile: %v", err)
			}
		}
	}

	// General settings update (theme, language, timezone → user_preferences).
	if in.Settings != nil && (in.Settings.Theme != "" || in.Settings.Language != "" || in.Settings.Timezone != "") {
		// Fetch current preferences to preserve fields not being updated.
		// No row yet means the schema defaults are in effect (the upsert below
		// creates the row).
		theme, language, timezone := "system", "en", "UTC"
		current, err := l.svcCtx.Repo.UserPreferences.GetUserPreferences(ctx, userID)
		if err == nil {
			theme, language, timezone = current.Theme, current.Language, current.Timezone
		} else if !errors.Is(err, pgx.ErrNoRows) {
			l.Errorf("Failed to fetch user preferences: %v", err)
			return nil, status.Error(codes.Internal, "failed to fetch user preferences")
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
		_, err = l.svcCtx.Repo.UserPreferences.UpdateUserPreferences(ctx, userID, theme, language, timezone)
		if err != nil {
			l.Errorf("Failed to update user preferences: %v", err)
			return nil, status.Error(codes.Internal, "failed to update user preferences")
		}
	}

	// Fire-and-forget publish settings/onboarding events to Kafka.
	if l.svcCtx.EventsPub != nil {
		if in.Settings != nil && in.Settings.OnboardingCompleted {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				env, err := events.NewEnvelope(events.TypeUserOnboarded, events.UserOnboarded{
					UserID: userID.String(),
				})
				if err != nil {
					logx.Errorf("envelope: %v", err)
					return
				}
				if err := l.svcCtx.EventsPub.Publish(ctx, env); err != nil {
					logx.Errorf("publish onboarding event: %v", err)
				}
			}()
		}

		if in.Settings != nil && (in.Settings.Timezone != "" || in.Settings.CheckInTime != "") {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				env, err := events.NewEnvelope(events.TypeSettingsChanged, events.SettingsChanged{
					UserID:      userID.String(),
					Timezone:    in.Settings.Timezone,
					CheckInTime: in.Settings.CheckInTime,
				})
				if err != nil {
					logx.Errorf("envelope: %v", err)
					return
				}
				if err := l.svcCtx.EventsPub.Publish(ctx, env); err != nil {
					logx.Errorf("publish settings event: %v", err)
				}
			}()
		}
	}

	return &client.UpdateSettingsResponse{
		Success: true,
	}, nil
}
