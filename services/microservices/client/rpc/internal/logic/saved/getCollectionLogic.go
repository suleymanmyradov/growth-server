package savedlogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

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

func (l *GetCollectionLogic) GetCollection(in *client.GetCollectionRequest) (*client.GetCollectionResponse, error) {
	l.Logger.Infof("Getting collection %s", in.CollectionId)

	return &client.GetCollectionResponse{
		Collection: &client.Collection{},
	}, nil
}
