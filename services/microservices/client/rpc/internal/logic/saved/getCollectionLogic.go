package savedlogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetCollectionLogic.GetCollection")
	defer span.End()
	logx.WithContext(ctx).Infof("Getting collection %s", in.CollectionId)

	return &client.GetCollectionResponse{
		Collection: &client.Collection{},
	}, nil
}
