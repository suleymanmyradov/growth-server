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

type HasCheckedInTodayLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewHasCheckedInTodayLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HasCheckedInTodayLogic {
	return &HasCheckedInTodayLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *HasCheckedInTodayLogic) HasCheckedInToday(req *types.HasCheckedInTodayRequest) (*types.HasCheckedInTodayResponse, error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	rpcResp, err := l.svcCtx.CheckInRpc.HasCheckedInToday(l.ctx, &clientcheckin.HasCheckedInTodayRequest{
		UserId:  p.UserID,
		HabitId: req.HabitId,
	})
	if err != nil {
		return nil, err
	}

	return &types.HasCheckedInTodayResponse{
		CheckedIn: rpcResp.CheckedIn,
	}, nil
}
