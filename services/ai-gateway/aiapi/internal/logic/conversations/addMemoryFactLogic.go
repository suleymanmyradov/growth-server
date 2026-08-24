// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package conversations

import (
	"context"
	"strings"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/types"

	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/aicoachservice"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AddMemoryFactLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddMemoryFactLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddMemoryFactLogic {
	return &AddMemoryFactLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AddMemoryFact stores something the user asked the coach to remember, or
// corrects something it had wrong. User-authored facts outrank model-extracted
// ones: on the subject of themselves, the user is authoritative.
func (l *AddMemoryFactLogic) AddMemoryFact(req *types.AddMemoryFactRequest) (resp *types.AddMemoryFactResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	if strings.TrimSpace(req.Fact) == "" {
		return nil, status.Error(codes.InvalidArgument, "fact is required")
	}

	rpcResp, err := l.svcCtx.AICoachRpc.AICoachService.AddUserFact(l.ctx, &aicoachservice.AddUserFactRequest{
		UserId:       p.UserID,
		Fact:         req.Fact,
		Category:     req.Category,
		SupersedesId: req.SupersedesId,
	})
	if err != nil {
		return nil, err
	}
	// A nil fact means it was already remembered verbatim -- idempotent from
	// the user's point of view, so not an error.
	if rpcResp.Fact == nil {
		return &types.AddMemoryFactResponse{}, nil
	}

	return &types.AddMemoryFactResponse{
		Data: types.MemoryFact{
			Id:           rpcResp.Fact.Id,
			Fact:         rpcResp.Fact.Fact,
			Category:     rpcResp.Fact.Category,
			Confidence:   rpcResp.Fact.Confidence,
			UserAuthored: rpcResp.Fact.UserAuthored,
			CreatedAt:    formatTime(rpcResp.Fact.CreatedAt),
		},
	}, nil
}
