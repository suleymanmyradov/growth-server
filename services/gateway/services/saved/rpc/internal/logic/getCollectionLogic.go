package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/saved/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/saved/rpc/saved"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetCollectionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCollectionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCollectionLogic {
	return &GetCollectionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetCollectionLogic) GetCollection(in *saved.GetCollectionRequest) (*saved.GetCollectionResponse, error) {
	// todo: add your logic here and delete this line

	return &saved.GetCollectionResponse{}, nil
}
