package personalizationservicelogic

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetCoachingProfileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCoachingProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCoachingProfileLogic {
	return &GetCoachingProfileLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetCoachingProfileLogic) GetCoachingProfile(in *client.GetCoachingProfileRequest) (*client.GetCoachingProfileResponse, error) {
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	profile, err := l.svcCtx.Repo.CoachingProfiles.GetCoachingProfile(l.ctx, userID)
	if err != nil {
		// If profile doesn't exist, return a default profile
		if errors.Is(err, pgx.ErrNoRows) {
			return &client.GetCoachingProfileResponse{
				Profile: &client.CoachingProfile{
					UserId:               in.UserId,
					AccountabilityStyle:  "balanced",
					PreferredTone:            "supportive",
					DifficultyPreference:           "adaptive",
					CommonBlockers:       []string{},
					CoachingNotesJson:    "{}",
				},
			}, nil
		}
		l.Errorf("failed to get coaching profile: %v", err)
		return nil, status.Error(codes.Internal, "failed to get coaching profile")
	}

	return &client.GetCoachingProfileResponse{
		Profile: dbCoachingProfileToProto(profile),
	}, nil
}

func dbCoachingProfileToProto(profile db.GetCoachingProfileRow) *client.CoachingProfile {
	var commonBlockers []string
	if profile.CommonBlockers != nil {
		if err := json.Unmarshal(profile.CommonBlockers, &commonBlockers); err != nil {
			logx.Errorf("failed to unmarshal common blockers: %v", err)
		}
	}

	coachingNotesJson := "{}"
	if profile.CoachingNotes != nil {
		coachingNotesJson = string(profile.CoachingNotes)
	}

	var lastContextRefreshAt int64 = 0
	if profile.LastContextRefreshAt.Valid {
		lastContextRefreshAt = profile.LastContextRefreshAt.Time.Unix()
	}

	primaryMotivation := ""
	if profile.PrimaryMotivation != nil {
		primaryMotivation = *profile.PrimaryMotivation
	}

	return &client.CoachingProfile{
		Id:                   profile.UserID.String(),
		UserId:               profile.UserID.String(),
		AccountabilityStyle:  string(profile.AccountabilityStyle),
		PreferredTone:            string(profile.PreferredTone),
		DifficultyPreference:           string(profile.DifficultyPreference),
		PrimaryMotivation:    primaryMotivation,
		CommonBlockers:       commonBlockers,
		CoachingNotesJson:    coachingNotesJson,
		LastContextRefreshAt: lastContextRefreshAt,
		CreatedAt:            profile.CreatedAt.Time.Unix(),
		UpdatedAt:            profile.UpdatedAt.Time.Unix(),
	}
}
