package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/conversations/rpc/conversations"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/conversations/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type StartConversationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewStartConversationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StartConversationLogic {
	return &StartConversationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *StartConversationLogic) StartConversation(in *conversations.StartConversationRequest) (*conversations.StartConversationResponse, error) {
	// todo: add your logic here and delete this line

	return &conversations.StartConversationResponse{}, nil
}
