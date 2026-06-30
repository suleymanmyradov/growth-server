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

type UnarchiveConversationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUnarchiveConversationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnarchiveConversationLogic {
	return &UnarchiveConversationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UnarchiveConversationLogic) UnarchiveConversation(req *types.ConversationRequest) (resp *types.GetConversationResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	rpcResp, err := l.svcCtx.ConversationRpc.UnarchiveConversation(l.ctx, &conversationservice.ArchiveConversationRequest{
		ConversationId: req.Id,
		UserId:         p.UserID,
	})
	if err != nil {
		return nil, err
	}

	return &types.GetConversationResponse{
		Data: types.Conversation{
			Id:          rpcResp.Conversation.Id,
			Title:       rpcResp.Conversation.Title,
			Type:        rpcResp.Conversation.Type,
			LastMessage: rpcResp.Conversation.LastMessage,
			UserId:      rpcResp.Conversation.UserId,
			Archived:    rpcResp.Conversation.Archived,
			CreatedAt:   formatTime(rpcResp.Conversation.CreatedAt),
			UpdatedAt:   formatTime(rpcResp.Conversation.UpdatedAt),
		},
	}, nil
}
