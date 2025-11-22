package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/conversations/rpc/conversations"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/conversations/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ContinueConversationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewContinueConversationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ContinueConversationLogic {
	return &ContinueConversationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ContinueConversationLogic) ContinueConversation(in *conversations.ContinueConversationRequest) (*conversations.ContinueConversationResponse, error) {
	// todo: add your logic here and delete this line

	return &conversations.ContinueConversationResponse{}, nil
}
