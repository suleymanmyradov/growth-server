package articleslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type GetFeaturedArticleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetFeaturedArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFeaturedArticleLogic {
	return &GetFeaturedArticleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetFeaturedArticleLogic) GetFeaturedArticle(in *client.GetFeaturedArticleRequest) (*client.GetFeaturedArticleResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetFeaturedArticleLogic.GetFeaturedArticle")
	defer span.End()

	article, err := l.svcCtx.Repo.Articles.GetFeaturedArticle(ctx)
	if err != nil {
		// No featured article found (e.g. empty table) — return an empty response
		// so the gateway can render a fallback. We don't treat this as an error.
		return &client.GetFeaturedArticleResponse{}, nil
	}

	pbArticle := convertFeaturedRowToPbArticle(article)

	// Hydrate tags, matching the GetArticle flow.
	tagRows, err := l.svcCtx.Repo.Articles.GetTagsByArticleIDs(ctx, []uuid.UUID{article.ID})
	if err != nil {
		l.Errorf("failed to get featured article tags: %v", err)
	} else {
		pbArticle.Tags = make([]string, 0, len(tagRows))
		for _, t := range tagRows {
			pbArticle.Tags = append(pbArticle.Tags, t.Name)
		}
	}

	return &client.GetFeaturedArticleResponse{
		Article: pbArticle,
	}, nil
}
