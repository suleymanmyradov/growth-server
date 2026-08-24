// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package conversations

import (
	"context"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/types"

	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/aicoachservice"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ForgetMemoryFactLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewForgetMemoryFactLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ForgetMemoryFactLogic {
	return &ForgetMemoryFactLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ForgetMemoryFact deletes one remembered fact.
func (l *ForgetMemoryFactLogic) ForgetMemoryFact(req *types.ForgetMemoryFactRequest) (resp *types.ForgetMemoryFactResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "fact id is required")
	}

	if _, err := l.svcCtx.AICoachRpc.AICoachService.ForgetUserFact(l.ctx, &aicoachservice.ForgetUserFactRequest{
		UserId: p.UserID,
		FactId: req.Id,
	}); err != nil {
		return nil, err
	}
	return &types.ForgetMemoryFactResponse{}, nil
}
