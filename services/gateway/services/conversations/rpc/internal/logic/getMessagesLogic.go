package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/conversations/rpc/conversations"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/conversations/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMessagesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMessagesLogic {
	return &GetMessagesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetMessagesLogic) GetMessages(in *conversations.GetMessagesRequest) (*conversations.GetMessagesResponse, error) {
	// todo: add your logic here and delete this line

	return &conversations.GetMessagesResponse{}, nil
}
