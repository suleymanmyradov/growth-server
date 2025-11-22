package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/settings/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/settings/rpc/settings"

	"github.com/zeromicro/go-zero/core/logx"
)

type ExportDataLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewExportDataLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExportDataLogic {
	return &ExportDataLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Data management
func (l *ExportDataLogic) ExportData(in *settings.ExportDataRequest) (*settings.ExportDataResponse, error) {
	// todo: add your logic here and delete this line

	return &settings.ExportDataResponse{}, nil
}
