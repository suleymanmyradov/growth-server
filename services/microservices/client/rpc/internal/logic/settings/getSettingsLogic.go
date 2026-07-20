package settingslogic

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetSettingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSettingsLogic {
	return &GetSettingsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetSettingsLogic) GetSettings(in *client.GetSettingsRequest) (*client.GetSettingsResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetSettingsLogic.GetSettings")
	defer span.End()

	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("failed to parse user ID: %v", err)
		return nil, status.Error(codes.Internal, "invalid user ID")
	}

	prefs, err := l.svcCtx.Repo.UserPreferences.GetUserPreferences(ctx, userID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			l.Errorf("failed to get user preferences: %v", err)
			return nil, status.Error(codes.Internal, "failed to get user settings")
		}
		// No preferences row yet (row is created lazily on first settings write):
		// fall back to the schema defaults instead of failing.
		return &client.GetSettingsResponse{
			Settings: &client.UserSettings{
				UserId:      userID.String(),
				Language:    "en",
				Theme:       "system",
				Timezone:    "UTC",
				CheckInTime: "09:00",
			},
		}, nil
	}

	// Compose the combined settings response from user_preferences + coaching_profiles.
	pb := &client.UserSettings{
		UserId:              prefs.UserID.String(),
		Language:            prefs.Language,
		Theme:               prefs.Theme,
		Timezone:            prefs.Timezone,
		OnboardingCompleted: prefs.OnboardingCompleted,
	}
	if prefs.CheckInTime.Valid {
		// pgtype.Time is microseconds since midnight; format as HH:MM without
		// involving time.Unix (which would apply the local timezone).
		secs := prefs.CheckInTime.Microseconds / 1_000_000
		pb.CheckInTime = fmt.Sprintf("%02d:%02d", secs/3600, (secs%3600)/60)
	}

	// Coaching profile (accountability_style) is optional.
	profile, err := l.svcCtx.Repo.CoachingProfiles.GetCoachingProfile(ctx, userID)
	if err == nil {
		pb.AccountabilityStyle = profile.AccountabilityStyle
	} else if !errors.Is(err, pgx.ErrNoRows) {
		l.Infof("failed to get coaching profile: %v", err)
	}

	return &client.GetSettingsResponse{
		Settings: pb,
	}, nil
}
