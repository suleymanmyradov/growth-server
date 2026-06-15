package settingslogic

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
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

	// Fetch current settings to get version for optimistic locking
	_, err = l.svcCtx.Repo.UserSettings.GetUserSettings(ctx, userID)
	if err != nil {
		l.Errorf("Failed to fetch user settings: %v", err)
return nil, status.Error(codes.Internal, "failed to fetch user settings")
	}

	// Handle onboarding settings update (accountability style, check-in time, onboarding flag)
	if in.Settings != nil && (in.Settings.AccountabilityStyle != "" || in.Settings.CheckInTime != "" || in.Settings.OnboardingCompleted) {
		style := in.Settings.AccountabilityStyle
		if style == "" {
			style = "balanced"
		}
		var checkInTime pgtype.Time
		if in.Settings.CheckInTime != "" {
			if t, err := time.Parse("15:04", in.Settings.CheckInTime); err == nil {
				checkInTime = pgtype.Time{Microseconds: int64(t.Hour()*3600000 + t.Minute()*60000), Valid: true}
			}
		}
		_, err = l.svcCtx.Repo.UserSettings.UpdateOnboardingSettings(ctx, userID, style, checkInTime, in.Settings.OnboardingCompleted)
		if err != nil {
			l.Errorf("Failed to update onboarding settings: %v", err)
return nil, status.Error(codes.Internal, "failed to update onboarding settings")
		}
	}

	params := db.UpdateUserSettingsParams{
		UserID: userID,
	}

	if in.Settings != nil {
		if in.Settings.Theme != "" {
			params.Theme = (in.Settings.Theme)
		}
		if in.Settings.Language != "" {
			params.Language = in.Settings.Language
		}
		if in.Settings.Timezone != "" {
			params.Timezone = in.Settings.Timezone
		}
		params.EmailNotifications = in.Settings.MarketingEmails
		params.PushNotifications = true
		params.HabitReminders = true
		params.GoalReminders = true
	}

	// Only run general settings update if there are non-onboarding fields to update
	if in.Settings != nil && (in.Settings.Theme != "" || in.Settings.Language != "" || in.Settings.Timezone != "") {
		_, err = l.svcCtx.Repo.UserSettings.UpdateUserSettings(ctx, params)
		if err != nil {
			l.Errorf("Failed to update user settings: %v", err)
return nil, status.Error(codes.Internal, "failed to update user settings")
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
					UserID:         userID.String(),
					Timezone:       in.Settings.Timezone,
					CheckInTime:    in.Settings.CheckInTime,
					HabitReminders: true,
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
