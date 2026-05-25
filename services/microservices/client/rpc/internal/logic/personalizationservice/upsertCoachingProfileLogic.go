package personalizationservicelogic

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpsertCoachingProfileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertCoachingProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertCoachingProfileLogic {
	return &UpsertCoachingProfileLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpsertCoachingProfileLogic) UpsertCoachingProfile(in *client.UpsertCoachingProfileRequest) (*client.UpsertCoachingProfileResponse, error) {
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	// Convert common blockers to JSON
	commonBlockersJSON, err := json.Marshal(in.CommonBlockers)
	if err != nil {
		l.Errorf("failed to marshal common blockers: %v", err)
		return nil, status.Error(codes.Internal, "failed to process common blockers")
	}

	// Convert coaching notes to JSON
	var coachingNotesJSON json.RawMessage
	if in.CoachingNotesJson != "" {
		coachingNotesJSON = json.RawMessage(in.CoachingNotesJson)
	} else {
		coachingNotesJSON = json.RawMessage("{}")
	}

	var primaryMotivation sql.NullString
	if in.PrimaryMotivation != "" {
		primaryMotivation = sql.NullString{String: in.PrimaryMotivation, Valid: true}
	}

	profile, err := l.svcCtx.Repo.CoachingProfiles.UpsertCoachingProfile(l.ctx, db.UpsertCoachingProfileParams{
		UserID:               userID,
		AccountabilityStyle:  in.AccountabilityStyle,
		PreferredTone:        in.PreferredTone,
		DifficultyPreference: in.DifficultyPreference,
		PrimaryMotivation:    primaryMotivation,
		CommonBlockers:       commonBlockersJSON,
		CoachingNotes:        coachingNotesJSON,
	})
	if err != nil {
		l.Errorf("failed to upsert coaching profile: %v", err)
		return nil, status.Error(codes.Internal, "failed to upsert coaching profile")
	}

	return &client.UpsertCoachingProfileResponse{
		Profile: dbCoachingProfileToProto(profile),
	}, nil
}
