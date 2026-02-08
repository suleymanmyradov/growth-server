package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/pb/search"

	"github.com/zeromicro/go-zero/core/logx"
)

type SaveSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSaveSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveSearchLogic {
	return &SaveSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SaveSearchLogic) SaveSearch(in *search.SaveSearchRequest) (*search.SaveSearchResponse, error) {
	// todo: add your logic here and delete this line

	return &search.SaveSearchResponse{}, nil
}
