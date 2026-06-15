package savedlogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type CreateCollectionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateCollectionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateCollectionLogic {
	return &CreateCollectionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateCollectionLogic) CreateCollection(in *client.CreateCollectionRequest) (*client.CreateCollectionResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "CreateCollectionLogic.CreateCollection")
	defer span.End()
	p, ok := principal.PrincipalFrom(ctx)
	if ok {
		l.Infof("Creating collection %s for user %s", in.Name, p.UserID)
	}

	return &client.CreateCollectionResponse{
		CollectionId: "",
	}, nil
}
