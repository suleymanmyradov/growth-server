// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package conversations

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListConversationsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListConversationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListConversationsLogic {
	return &ListConversationsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListConversationsLogic) ListConversations(req *types.ListConversationsRequest) (resp *types.ConversationsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
