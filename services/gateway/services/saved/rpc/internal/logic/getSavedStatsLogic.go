package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/saved/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/saved/rpc/saved"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSavedStatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSavedStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSavedStatsLogic {
	return &GetSavedStatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Analytics
func (l *GetSavedStatsLogic) GetSavedStats(in *saved.GetSavedStatsRequest) (*saved.GetSavedStatsResponse, error) {
	// todo: add your logic here and delete this line

	return &saved.GetSavedStatsResponse{}, nil
}
