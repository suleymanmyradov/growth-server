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

type StartConversationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewStartConversationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StartConversationLogic {
	return &StartConversationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *StartConversationLogic) StartConversation(req *types.StartConversationRequest) (resp *types.StartConversationResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	convType := req.Type
	if convType == "" {
		convType = "coach"
	}

	rpcResp, err := l.svcCtx.ConversationRpc.StartConversation(l.ctx, &conversationservice.StartConversationRequest{
		UserId:          p.UserID,
		Type:            convType,
		Title:           req.Title,
		InitialMessage:  req.InitialMessage,
	})
	if err != nil {
		return nil, err
	}

	resp = &types.StartConversationResponse{
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
	}

	if rpcResp.InitialMessageRow != nil {
		resp.InitialMessage = &types.ConversationMessage{
			Id:             rpcResp.InitialMessageRow.Id,
			ConversationId: rpcResp.InitialMessageRow.ConversationId,
			Role:           rpcResp.InitialMessageRow.Role,
			Content:        rpcResp.InitialMessageRow.Content,
			CreatedAt:      formatTime(rpcResp.InitialMessageRow.CreatedAt),
		}
	}

	return resp, nil
}
