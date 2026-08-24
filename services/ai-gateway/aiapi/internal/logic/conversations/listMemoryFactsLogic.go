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

type ListMemoryFactsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListMemoryFactsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMemoryFactsLogic {
	return &ListMemoryFactsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListMemoryFacts returns everything the coach currently remembers about the
// caller, low-confidence entries included. A user who cannot see a shaky guess
// cannot correct it.
func (l *ListMemoryFactsLogic) ListMemoryFacts(req *types.ListMemoryFactsRequest) (resp *types.ListMemoryFactsResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	limit := req.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	rpcResp, err := l.svcCtx.AICoachRpc.AICoachService.ListUserFacts(l.ctx, &aicoachservice.ListUserFactsRequest{
		UserId: p.UserID,
		Page:   int32(page),
		Limit:  int32(limit),
	})
	if err != nil {
		return nil, err
	}

	facts := make([]types.MemoryFact, 0, len(rpcResp.Facts))
	for _, f := range rpcResp.Facts {
		facts = append(facts, types.MemoryFact{
			Id:           f.Id,
			Fact:         f.Fact,
			Category:     f.Category,
			Confidence:   f.Confidence,
			UserAuthored: f.UserAuthored,
			CreatedAt:    formatTime(f.CreatedAt),
		})
	}

	totalPages := 0
	if rpcResp.Total > 0 {
		totalPages = (int(rpcResp.Total) + limit - 1) / limit
	}

	return &types.ListMemoryFactsResponse{
		Data: facts,
		Page: types.PageResponse{
			Page:       page,
			Limit:      limit,
			Total:      int64(rpcResp.Total),
			TotalPages: totalPages,
		},
	}, nil
}
