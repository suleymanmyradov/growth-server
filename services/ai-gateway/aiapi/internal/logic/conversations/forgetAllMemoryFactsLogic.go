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

type ForgetAllMemoryFactsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewForgetAllMemoryFactsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ForgetAllMemoryFactsLogic {
	return &ForgetAllMemoryFactsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ForgetAllMemoryFacts wipes everything the coach remembers about the caller.
//
// Requires ?confirm=true. Without the guard a stray DELETE on the collection
// URL -- a mistyped path, an over-eager client retry -- would silently erase
// memory the user spent months building, with no undo.
func (l *ForgetAllMemoryFactsLogic) ForgetAllMemoryFacts(req *types.ForgetAllMemoryFactsRequest) (resp *types.ForgetAllMemoryFactsResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	if !req.Confirm {
		return nil, status.Error(codes.InvalidArgument, "pass confirm=true to clear everything the coach remembers")
	}

	if _, err := l.svcCtx.AICoachRpc.AICoachService.ForgetAllUserFacts(l.ctx, &aicoachservice.ForgetAllUserFactsRequest{
		UserId: p.UserID,
	}); err != nil {
		return nil, err
	}
	return &types.ForgetAllMemoryFactsResponse{Forgotten: true}, nil
}
