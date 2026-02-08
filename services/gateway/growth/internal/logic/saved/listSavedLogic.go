// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package saved

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSavedLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListSavedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSavedLogic {
	return &ListSavedLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListSavedLogic) ListSaved(req *types.PageRequest) (resp *types.SavedItemsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
