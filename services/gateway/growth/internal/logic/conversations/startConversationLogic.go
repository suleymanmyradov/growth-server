// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package conversations

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
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

func (l *StartConversationLogic) StartConversation(req *types.StartConversationRequest) (resp *types.ConversationResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
