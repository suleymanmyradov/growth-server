// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package personalization

import (
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	"context"
	"encoding/json"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientpersonalization "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/personalizationservice"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpsertCoachingProfileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpsertCoachingProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertCoachingProfileLogic {
	return &UpsertCoachingProfileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpsertCoachingProfileLogic) UpsertCoachingProfile(req *types.UpsertCoachingProfileRequest) (resp *types.CoachingProfileResponse, err error) {
	principal, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	coachingNotes, err := json.Marshal(req.CoachingNotes)
	if err != nil {
		return nil, err
	}

	rpcResp, err := l.svcCtx.PersonalizationRpc.UpsertCoachingProfile(l.ctx, &clientpersonalization.UpsertCoachingProfileRequest{
		UserId:               principal.UserID,
		AccountabilityStyle:  req.AccountabilityStyle,
		PreferredTone:        req.PreferredTone,
		DifficultyPreference: req.DifficultyPreference,
		CommonBlockers:       req.CommonBlockers,
		CoachingNotesJson:    string(coachingNotes),
	})
	if err != nil {
		return nil, err
	}

	// Parse coaching notes JSON
	var coachingNotesOut map[string]string
	if rpcResp.Profile.CoachingNotesJson != "" {
		if err := json.Unmarshal([]byte(rpcResp.Profile.CoachingNotesJson), &coachingNotesOut); err != nil {
			coachingNotesOut = make(map[string]string)
		}
	} else {
		coachingNotesOut = make(map[string]string)
	}

	return &types.CoachingProfileResponse{
		Data: types.CoachingProfile{
			Id:                   rpcResp.Profile.Id,
			UserId:               rpcResp.Profile.UserId,
			AccountabilityStyle:  rpcResp.Profile.AccountabilityStyle,
			PreferredTone:        rpcResp.Profile.PreferredTone,
			DifficultyPreference: rpcResp.Profile.DifficultyPreference,
			PrimaryMotivation:    rpcResp.Profile.PrimaryMotivation,
			CommonBlockers:       rpcResp.Profile.CommonBlockers,
			CoachingNotes:        coachingNotesOut,
			LastContextRefreshAt: formatTimestamp(rpcResp.Profile.LastContextRefreshAt),
			CreatedAt:            formatTimestamp(rpcResp.Profile.CreatedAt),
			UpdatedAt:            formatTimestamp(rpcResp.Profile.UpdatedAt),
		},
	}, nil
}
