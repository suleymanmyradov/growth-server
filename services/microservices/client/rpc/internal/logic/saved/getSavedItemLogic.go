package savedlogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type GetSavedItemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSavedItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSavedItemLogic {
	return &GetSavedItemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetSavedItemLogic) GetSavedItem(in *client.GetSavedItemRequest) (*client.GetSavedItemResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetSavedItemLogic.GetSavedItem")
	defer span.End()
	logx.WithContext(ctx).Infof("Getting saved item %s", in.SavedId)

	return &client.GetSavedItemResponse{
		Item: &client.SavedItem{},
	}, nil
}
