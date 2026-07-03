// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package personalization

import (
	"context"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientpersonalization "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/personalizationservice"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	// Fetch the personalization context from the client RPC (DB-backed).
	contextResp, err := l.svcCtx.ClientRpc.PersonalizationService.GetPersonalizationContext(l.ctx, &clientpersonalization.GetPersonalizationContextRequest{
		UserId:       p.UserID,
		ForceRefresh: false,
	})
	if err != nil {
		l.Errorf("failed to get personalization context: %v", err)
		return nil, status.Error(codes.Internal, "failed to get personalization context")
	}

	// Build the ai-coach request from the context + user message.
	aiReq := BuildPersonalizedCoachingRequest(p.UserID, req.UserMessage, nil, contextResp.Context)

	// Call the ai-coach RPC directly (gateway orchestrates cross-service flow).
	aiResp, aiErr := l.svcCtx.AICoachRpc.AICoachService.GeneratePersonalizedCoaching(l.ctx, aiReq)

	coachingResponse := ""
	if aiErr != nil {
		l.Errorf("AI generation failed: %v", aiErr)
		coachingResponse = "I couldn't generate a full coaching response right now, but based on your recent activity, pick one small action you can complete today and keep it easy. Small consistent actions build momentum over time."
	} else {
		coachingResponse = aiResp.CoachingResponse
	}

	return &types.GeneratePersonalizedCoachingResponse{
		CoachingResponse: coachingResponse,
		Context:          req.Context,
	}, nil
}
