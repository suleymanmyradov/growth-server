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

type GetCoachingProfileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetCoachingProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCoachingProfileLogic {
	return &GetCoachingProfileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetCoachingProfileLogic) GetCoachingProfile() (resp *types.CoachingProfileResponse, err error) {
	principal, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	rpcResp, err := l.svcCtx.PersonalizationRpc.GetCoachingProfile(l.ctx, &clientpersonalization.GetCoachingProfileRequest{
		UserId: principal.UserID,
	})
	if err != nil {
		return nil, err
	}

	// Parse coaching notes JSON
	var coachingNotes map[string]string
	if rpcResp.Profile.CoachingNotesJson != "" {
		if err := json.Unmarshal([]byte(rpcResp.Profile.CoachingNotesJson), &coachingNotes); err != nil {
			coachingNotes = make(map[string]string)
		}
	} else {
		coachingNotes = make(map[string]string)
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
			CoachingNotes:        coachingNotes,
			LastContextRefreshAt: formatTimestamp(rpcResp.Profile.LastContextRefreshAt),
			CreatedAt:            formatTimestamp(rpcResp.Profile.CreatedAt),
			UpdatedAt:            formatTimestamp(rpcResp.Profile.UpdatedAt),
		},
	}, nil
}
