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

type ArchiveConversationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewArchiveConversationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ArchiveConversationLogic {
	return &ArchiveConversationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ArchiveConversationLogic) ArchiveConversation(req *types.ConversationRequest) (resp *types.GetConversationResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	rpcResp, err := l.svcCtx.ConversationRpc.ArchiveConversation(l.ctx, &conversationservice.ArchiveConversationRequest{
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
