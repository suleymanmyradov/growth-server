package settingslogic

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
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
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	// Handle onboarding settings update (accountability style, check-in time, onboarding flag)
	if in.Settings != nil && (in.Settings.AccountabilityStyle != "" || in.Settings.CheckInTime != "" || in.Settings.OnboardingCompleted) {
		style := in.Settings.AccountabilityStyle
		if style == "" {
			style = "balanced"
		}
		checkInTime := sql.NullTime{}
		if in.Settings.CheckInTime != "" {
			if t, err := time.Parse("15:04", in.Settings.CheckInTime); err == nil {
				checkInTime = sql.NullTime{Time: t, Valid: true}
			}
		}
		onboardingParams := db.UpdateOnboardingSettingsParams{
			UserID:              userID,
			AccountabilityStyle: style,
			CheckInTime:         checkInTime,
			OnboardingCompleted: in.Settings.OnboardingCompleted,
		}
		_, err = l.svcCtx.Repo.UserSettings.UpdateOnboardingSettings(l.ctx, onboardingParams)
		if err != nil {
			l.Errorf("Failed to update onboarding settings: %v", err)
			return nil, err
		}
	}

	params := db.UpdateUserSettingsParams{
		UserID: userID,
	}

	if in.Settings != nil {
		if in.Settings.Theme != "" {
			params.Theme = in.Settings.Theme
		}
		if in.Settings.Language != "" {
			params.Language = in.Settings.Language
		}
		if in.Settings.Timezone != "" {
			params.Timezone = in.Settings.Timezone
		}
		params.EmailNotifications = sql.NullBool{Bool: in.Settings.MarketingEmails, Valid: true}
		params.PushNotifications = sql.NullBool{Bool: true, Valid: true}
		params.HabitReminders = sql.NullBool{Bool: true, Valid: true}
		params.GoalReminders = sql.NullBool{Bool: true, Valid: true}
	}

	// Only run general settings update if there are non-onboarding fields to update
	if in.Settings != nil && (in.Settings.Theme != "" || in.Settings.Language != "" || in.Settings.Timezone != "") {
		_, err = l.svcCtx.Repo.UserSettings.UpdateUserSettings(l.ctx, params)
		if err != nil {
			l.Errorf("Failed to update user settings: %v", err)
			return nil, err
		}
	}

	return &client.UpdateSettingsResponse{
		Success: true,
	}, nil
}
