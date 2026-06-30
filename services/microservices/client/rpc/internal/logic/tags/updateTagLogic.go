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

type UpdateTagLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateTagLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateTagLogic {
	return &UpdateTagLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateTagLogic) UpdateTag(in *client.UpdateTagRequest) (*client.UpdateTagResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "UpdateTagLogic.UpdateTag")
	defer span.End()
	id, err := uuid.Parse(in.TagId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tag id")
	}
	if in.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	slug := in.Slug
	if slug == "" {
		return nil, status.Error(codes.InvalidArgument, "slug is required")
	}

	tag, err := l.svcCtx.Repo.Tags.UpdateTag(ctx, id, in.Name, slug)
	if err != nil {
		l.Errorf("update tag failed: %v", err)
		return nil, status.Error(codes.Internal, "update tag failed")
	}

	return &client.UpdateTagResponse{Tag: convertTag(tag)}, nil
}
