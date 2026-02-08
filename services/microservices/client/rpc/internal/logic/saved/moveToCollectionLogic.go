package savedlogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

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

func (l *MoveToCollectionLogic) MoveToCollection(in *client.MoveToCollectionRequest) (*client.MoveToCollectionResponse, error) {
	l.Logger.Infof("Moving saved item %s to collection %s", in.SavedId, in.CollectionId)

	return &client.MoveToCollectionResponse{
		Success: true,
	}, nil
}
