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

type GetMessagesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMessagesLogic {
	return &GetMessagesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetMessagesLogic) GetMessages(req *types.GetMessagesRequest) (resp *types.GetMessagesResponse, err error) {
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

	rpcResp, err := l.svcCtx.ConversationRpc.GetMessages(l.ctx, &conversationservice.GetMessagesRequest{
		ConversationId: req.Id,
		UserId:         p.UserID,
		Page:           int32(page),
		Limit:          int32(limit),
	})
	if err != nil {
		return nil, err
	}

	msgs := make([]types.ConversationMessage, 0, len(rpcResp.Messages))
	for _, m := range rpcResp.Messages {
		msgs = append(msgs, types.ConversationMessage{
			Id:             m.Id,
			ConversationId: m.ConversationId,
			Role:           m.Role,
			Content:        m.Content,
			CreatedAt:      formatTime(m.CreatedAt),
		})
	}

	totalPages := int64(0)
	if rpcResp.Total > 0 {
		totalPages = (int64(rpcResp.Total) + int64(limit) - 1) / int64(limit)
	}

	return &types.GetMessagesResponse{
		Data: msgs,
		Page: types.PageResponse{
			Total:      int64(rpcResp.Total),
			Page:       page,
			Limit:      limit,
			TotalPages: int(totalPages),
		},
	}, nil
}
