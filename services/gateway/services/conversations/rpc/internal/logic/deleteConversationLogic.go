package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/conversations/rpc/conversations"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/conversations/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteConversationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteConversationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteConversationLogic {
	return &DeleteConversationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteConversationLogic) DeleteConversation(in *conversations.DeleteConversationRequest) (*conversations.DeleteConversationResponse, error) {
	// todo: add your logic here and delete this line

	return &conversations.DeleteConversationResponse{}, nil
}
