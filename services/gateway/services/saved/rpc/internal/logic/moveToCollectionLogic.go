package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/saved/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/saved/rpc/saved"

	"github.com/zeromicro/go-zero/core/logx"
)

type MoveToCollectionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMoveToCollectionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MoveToCollectionLogic {
	return &MoveToCollectionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Collection operations
func (l *MoveToCollectionLogic) MoveToCollection(in *saved.MoveToCollectionRequest) (*saved.MoveToCollectionResponse, error) {
	// todo: add your logic here and delete this line

	return &saved.MoveToCollectionResponse{}, nil
}
