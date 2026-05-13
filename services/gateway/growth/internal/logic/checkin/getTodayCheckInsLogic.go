package checkin

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientcheckin "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/checkinservice"

	"errors"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetTodayCheckInsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetTodayCheckInsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTodayCheckInsLogic {
	return &GetTodayCheckInsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetTodayCheckInsLogic) GetTodayCheckIns(req *types.GetTodayCheckInsRequest) (resp *types.GetTodayCheckInsResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, errors.New("unauthenticated")
	}

	rpcResp, err := l.svcCtx.CheckInRpc.GetTodayCheckIns(l.ctx, &clientcheckin.GetTodayCheckInsRequest{
		UserId: p.UserID,
	})
	if err != nil {
		return nil, err
	}

	var checkIns []types.CheckIn
	for _, ci := range rpcResp.CheckIns {
		checkIns = append(checkIns, types.CheckIn{
			Id:        ci.Id,
			UserId:    ci.UserId,
			HabitId:   ci.HabitId,
			Status:    ci.Status,
			Mood:      ci.Mood,
			Energy:    ci.Energy,
			Blocker:   ci.Blocker,
			Note:      ci.Note,
			CreatedAt: formatTime(ci.CreatedAt),
		})
	}

	return &types.GetTodayCheckInsResponse{
		CheckIns: checkIns,
	}, nil
}
