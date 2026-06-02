package checkin

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientcheckin "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/checkinservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetCheckInHistoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetCheckInHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCheckInHistoryLogic {
	return &GetCheckInHistoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetCheckInHistoryLogic) GetCheckInHistory(req *types.GetCheckInHistoryRequest) (resp *types.GetCheckInHistoryResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	rpcResp, err := l.svcCtx.CheckInRpc.GetCheckInHistory(l.ctx, &clientcheckin.GetCheckInHistoryRequest{
		UserId:  p.UserID,
		HabitId: req.HabitId,
		Page:    int32(req.Page),
		Limit:   int32(req.Limit),
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

	return &types.GetCheckInHistoryResponse{
		Data: checkIns,
		Page: types.PageResponse{
			Total:      int64(rpcResp.Total),
			Page:       req.Page,
			Limit:      req.Limit,
			TotalPages: int((int64(rpcResp.Total) + int64(req.Limit) - 1) / int64(req.Limit)),
		},
	}, nil
}
