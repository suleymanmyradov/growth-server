package personalizationservicelogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdateCoachingProfilePreferencesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateCoachingProfilePreferencesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateCoachingProfilePreferencesLogic {
	return &UpdateCoachingProfilePreferencesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateCoachingProfilePreferencesLogic) UpdateCoachingProfilePreferences(in *client.UpdateCoachingProfilePreferencesRequest) (*client.UpdateCoachingProfilePreferencesResponse, error) {
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	// Validate enum values
	validAccountabilityStyles := map[string]bool{
		"gentle":   true,
		"balanced": true,
		"strict":   true,
	}
	if !validAccountabilityStyles[in.AccountabilityStyle] {
		return nil, status.Error(codes.InvalidArgument, "invalid accountability style")
	}

	validTones := map[string]bool{
		"supportive":  true,
		"direct":      true,
		"warm":        true,
		"practical":   true,
		"challenging": true,
	}
	if !validTones[in.PreferredTone] {
		return nil, status.Error(codes.InvalidArgument, "invalid preferred tone")
	}

	validDifficulties := map[string]bool{
		"easy":      true,
		"adaptive":  true,
		"ambitious": true,
	}
	if !validDifficulties[in.DifficultyPreference] {
		return nil, status.Error(codes.InvalidArgument, "invalid difficulty preference")
	}

	// Update coaching profile preferences
	profile, err := l.svcCtx.Repo.CoachingProfiles.UpdateCoachingProfilePreferences(l.ctx, db.UpdateCoachingProfilePreferencesParams{
		UserID:               userID,
		AccountabilityStyle:  in.AccountabilityStyle,
		PreferredTone:        in.PreferredTone,
		DifficultyPreference: in.DifficultyPreference,
	})
	if err != nil {
		l.Errorf("failed to update coaching profile preferences: %v", err)
		return nil, status.Error(codes.Internal, "failed to update coaching profile preferences")
	}

	return &client.UpdateCoachingProfilePreferencesResponse{
		Profile: dbCoachingProfileToProto(profile),
	}, nil
}
