package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/saved/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/saved/rpc/saved"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateCollectionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateCollectionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateCollectionLogic {
	return &UpdateCollectionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateCollectionLogic) UpdateCollection(in *saved.UpdateCollectionRequest) (*saved.UpdateCollectionResponse, error) {
	// todo: add your logic here and delete this line

	return &saved.UpdateCollectionResponse{}, nil
}
