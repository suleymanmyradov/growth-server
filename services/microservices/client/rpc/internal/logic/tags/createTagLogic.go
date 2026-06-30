package tagslogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateTagLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateTagLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateTagLogic {
	return &CreateTagLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateTagLogic) CreateTag(in *client.CreateTagRequest) (*client.CreateTagResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "CreateTagLogic.CreateTag")
	defer span.End()
	if in.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	slug := in.Slug
	if slug == "" {
		return nil, status.Error(codes.InvalidArgument, "slug is required")
	}

	tag, err := l.svcCtx.Repo.Tags.CreateTag(ctx, in.Name, slug)
	if err != nil {
		l.Errorf("create tag failed: %v", err)
		return nil, status.Error(codes.Internal, "create tag failed")
	}

	return &client.CreateTagResponse{Tag: convertTag(tag)}, nil
}
