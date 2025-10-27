// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package saved

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SaveItemLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSaveItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveItemLogic {
	return &SaveItemLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SaveItemLogic) SaveItem(req *types.SaveItemRequest) (resp *types.SavedItemResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
