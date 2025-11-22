package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/saved/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/saved/rpc/saved"

	"github.com/zeromicro/go-zero/core/logx"
)

type ExportSavedLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewExportSavedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExportSavedLogic {
	return &ExportSavedLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ExportSavedLogic) ExportSaved(in *saved.ExportSavedRequest) (*saved.ExportSavedResponse, error) {
	// todo: add your logic here and delete this line

	return &saved.ExportSavedResponse{}, nil
}
