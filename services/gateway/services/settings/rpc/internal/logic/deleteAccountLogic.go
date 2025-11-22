package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/settings/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/settings/rpc/settings"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteAccountLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteAccountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteAccountLogic {
	return &DeleteAccountLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteAccountLogic) DeleteAccount(in *settings.DeleteAccountRequest) (*settings.DeleteAccountResponse, error) {
	// todo: add your logic here and delete this line

	return &settings.DeleteAccountResponse{}, nil
}
