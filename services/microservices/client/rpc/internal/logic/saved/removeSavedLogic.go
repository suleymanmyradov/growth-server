package savedlogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type RemoveSavedLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRemoveSavedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RemoveSavedLogic {
	return &RemoveSavedLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RemoveSavedLogic) RemoveSaved(in *client.RemoveSavedRequest) (*client.RemoveSavedResponse, error) {
	l.Logger.Infof("Removing saved item %s", in.SavedId)

	return &client.RemoveSavedResponse{
		Success: true,
	}, nil
}
