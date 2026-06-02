// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package personalization

import (
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientpersonalization "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/personalizationservice"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
)

type GeneratePersonalizedCoachingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGeneratePersonalizedCoachingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GeneratePersonalizedCoachingLogic {
	return &GeneratePersonalizedCoachingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GeneratePersonalizedCoachingLogic) GeneratePersonalizedCoaching(req *types.GeneratePersonalizedCoachingRequest) (resp *types.GeneratePersonalizedCoachingResponse, err error) {
	principal, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	rpcResp, err := l.svcCtx.PersonalizationRpc.GeneratePersonalizedCoaching(l.ctx, &clientpersonalization.GeneratePersonalizedCoachingRequest{
		UserId:      principal.UserID,
		UserMessage: req.UserMessage,
		Context:     req.Context,
	})
	if err != nil {
		return nil, err
	}

	return &types.GeneratePersonalizedCoachingResponse{
		CoachingResponse: rpcResp.CoachingResponse,
		Context:          req.Context,
	}, nil
}
