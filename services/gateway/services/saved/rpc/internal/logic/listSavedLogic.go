package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/saved/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/saved/rpc/saved"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSavedLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSavedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSavedLogic {
	return &ListSavedLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Saved items CRUD
func (l *ListSavedLogic) ListSaved(in *saved.ListSavedRequest) (*saved.ListSavedResponse, error) {
	// todo: add your logic here and delete this line

	return &saved.ListSavedResponse{}, nil
}
