package tagslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DeleteTagLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteTagLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteTagLogic {
	return &DeleteTagLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteTagLogic) DeleteTag(in *client.DeleteTagRequest) (*client.DeleteTagResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "DeleteTagLogic.DeleteTag")
	defer span.End()
	id, err := uuid.Parse(in.TagId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tag id")
	}

	// article_tags has ON DELETE CASCADE, so linking rows are removed automatically.
	if err := l.svcCtx.Repo.Tags.DeleteTag(ctx, id); err != nil {
		l.Errorf("delete tag failed: %v", err)
		return nil, status.Error(codes.Internal, "delete tag failed")
	}

	return &client.DeleteTagResponse{Success: true}, nil
}
