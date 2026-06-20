package habits

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clienthabits "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/habits"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
)

type ResetTodayHabitsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewResetTodayHabitsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetTodayHabitsLogic {
	return &ResetTodayHabitsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ResetTodayHabitsLogic) ResetTodayHabits() (resp *types.EmptyResponse, err error) {
	_, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	_, err = l.svcCtx.HabitsRpc.ResetTodayHabits(l.ctx, &clienthabits.ResetTodayHabitsRequest{})
	if err != nil {
		return nil, err
	}

	return &types.EmptyResponse{}, nil
}
