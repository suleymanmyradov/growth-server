package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/saved/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/saved/rpc/saved"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListCollectionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListCollectionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCollectionsLogic {
	return &ListCollectionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListCollectionsLogic) ListCollections(in *saved.ListCollectionsRequest) (*saved.ListCollectionsResponse, error) {
	// todo: add your logic here and delete this line

	return &saved.ListCollectionsResponse{}, nil
}
