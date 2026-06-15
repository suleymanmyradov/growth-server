package savedlogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type ListCollectionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListCollectionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCollectionsLogic {
	return &ListCollectionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListCollectionsLogic) ListCollections(in *client.ListCollectionsRequest) (*client.ListCollectionsResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ListCollectionsLogic.ListCollections")
	defer span.End()
	p, ok := principal.PrincipalFrom(ctx)
	if ok {
		l.Infof("Listing collections for user %s", p.UserID)
	}

	return &client.ListCollectionsResponse{
		Collections: []*client.Collection{},
	}, nil
}
