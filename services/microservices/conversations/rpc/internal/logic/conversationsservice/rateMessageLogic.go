package conversationsservicelogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/conversations/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/conversations/rpc/pb/conversations"

	"github.com/zeromicro/go-zero/core/logx"
)

type RateMessageLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRateMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RateMessageLogic {
	return &RateMessageLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RateMessageLogic) RateMessage(in *conversations.RateMessageRequest) (*conversations.RateMessageResponse, error) {
	// todo: add your logic here and delete this line

	return &conversations.RateMessageResponse{}, nil
}
