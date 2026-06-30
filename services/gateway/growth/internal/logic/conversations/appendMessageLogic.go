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

type AppendMessageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAppendMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppendMessageLogic {
	return &AppendMessageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AppendMessageLogic) AppendMessage(req *types.AppendMessageRequest) (resp *types.AppendMessageResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	role := req.Role
	if role == "" {
		role = "user"
	}

	rpcResp, err := l.svcCtx.ConversationRpc.AppendMessage(l.ctx, &conversationservice.AppendMessageRequest{
		ConversationId: req.Id,
		UserId:         p.UserID,
		Role:           role,
		Content:        req.Content,
	})
	if err != nil {
		return nil, err
	}

	return &types.AppendMessageResponse{
		Data: types.ConversationMessage{
			Id:             rpcResp.Message.Id,
			ConversationId: rpcResp.Message.ConversationId,
			Role:           rpcResp.Message.Role,
			Content:        rpcResp.Message.Content,
			CreatedAt:      formatTime(rpcResp.Message.CreatedAt),
		},
		Conversation: types.Conversation{
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
