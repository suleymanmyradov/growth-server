package conversations

import (
	"context"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/conversationservice"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListConversationsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListConversationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListConversationsLogic {
	return &ListConversationsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListConversationsLogic) ListConversations(req *types.ListConversationsRequest) (resp *types.ListConversationsResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	rpcResp, err := l.svcCtx.ConversationRpc.ListConversations(l.ctx, &conversationservice.ListConversationsRequest{
		UserId: p.UserID,
		Type:   req.Type,
		Page:   int32(page),
		Limit:  int32(limit),
	})
	if err != nil {
		return nil, err
	}

	convs := make([]types.Conversation, 0, len(rpcResp.Conversations))
	for _, c := range rpcResp.Conversations {
		convs = append(convs, types.Conversation{
			Id:          c.Id,
			Title:       c.Title,
			Type:        c.Type,
			LastMessage: c.LastMessage,
			UserId:      c.UserId,
			Archived:    c.Archived,
			CreatedAt:   formatTime(c.CreatedAt),
			UpdatedAt:   formatTime(c.UpdatedAt),
		})
	}

	totalPages := int64(0)
	if rpcResp.Total > 0 {
		totalPages = (int64(rpcResp.Total) + int64(limit) - 1) / int64(limit)
	}

	return &types.ListConversationsResponse{
		Data: convs,
		Page: types.PageResponse{
			Total:      int64(rpcResp.Total),
			Page:       page,
			Limit:      limit,
			TotalPages: int(totalPages),
		},
	}, nil
}
