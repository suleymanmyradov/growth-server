// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package saved

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RemoveSavedLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRemoveSavedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RemoveSavedLogic {
	return &RemoveSavedLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RemoveSavedLogic) RemoveSaved(req *types.SavedItemRequest) (resp *types.EmptyResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
