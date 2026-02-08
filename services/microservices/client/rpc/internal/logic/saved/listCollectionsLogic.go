package savedlogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

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

func (l *ListCollectionsLogic) ListCollections(in *client.ListCollectionsRequest) (*client.ListCollectionsResponse, error) {
	l.Logger.Infof("Listing collections for user %s", in.UserId)

	return &client.ListCollectionsResponse{
		Collections: []*client.Collection{},
	}, nil
}
