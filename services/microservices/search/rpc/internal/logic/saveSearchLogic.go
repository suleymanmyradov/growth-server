package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/pb/search"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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

func (l *SaveSearchLogic) SaveSearch(_ *search.SaveSearchRequest) (*search.SaveSearchResponse, error) {
	_, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "SaveSearchLogic.SaveSearch")
	defer span.End()

	// todo: add your logic here and delete this line

	return &search.SaveSearchResponse{}, nil
}
