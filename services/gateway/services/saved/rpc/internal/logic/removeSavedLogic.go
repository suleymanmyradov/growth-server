package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/saved/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/saved/rpc/saved"

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

func (l *RemoveSavedLogic) RemoveSaved(in *saved.RemoveSavedRequest) (*saved.RemoveSavedResponse, error) {
	// todo: add your logic here and delete this line

	return &saved.RemoveSavedResponse{}, nil
}
