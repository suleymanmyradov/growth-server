package articleslogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type ListTagsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListTagsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTagsLogic {
	return &ListTagsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListTagsLogic) ListTags(in *client.ListTagsRequest) (*client.ListTagsResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ListTagsLogic.ListTags")
	defer span.End()

	tagRows, err := l.svcCtx.Repo.Articles.ListTags(ctx)
	if err != nil {
		l.Errorf("failed to list tags: %v", err)
		return nil, err
	}

	tags := make([]*client.Tag, 0, len(tagRows))
	for _, t := range tagRows {
		tags = append(tags, &client.Tag{
			Id:   t.ID.String(),
			Name: t.Name,
			Slug: t.Slug,
		})
	}

	return &client.ListTagsResponse{
		Tags: tags,
	}, nil
}
