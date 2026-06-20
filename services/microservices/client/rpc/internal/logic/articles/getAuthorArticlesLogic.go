package articleslogic

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type GetAuthorArticlesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAuthorArticlesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAuthorArticlesLogic {
	return &GetAuthorArticlesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetAuthorArticlesLogic) GetAuthorArticles(in *client.GetAuthorArticlesRequest) (*client.GetAuthorArticlesResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetAuthorArticlesLogic.GetAuthorArticles")
	defer span.End()
	limit := int32(20)
	offset := int32(0)
	if in.Limit > 0 {
		limit = in.Limit
	}
	if in.Offset > 0 {
		offset = in.Offset
	}

	userID, userErr := uuid.Parse(in.UserId)
	hasUser := in.UserId != "" && userErr == nil

	var pbArticles []*client.Article
	// Author pages are public — only published articles are visible.
	const publishedStatus = "published"
	if hasUser {
		articles, err := l.svcCtx.Repo.Articles.ListArticlesByAuthorWithSaved(ctx, in.AuthorId, publishedStatus, limit, offset, userID)
		if err != nil {
			l.Errorf("Failed to list author articles with saved: %v", err)
			return nil, status.Error(codes.Internal, "failed to list author articles")
		}
		pbArticles = make([]*client.Article, len(articles))
		for i, a := range articles {
			pbArticles[i] = convertAuthorWithSavedRowToPbArticle(a)
		}
	} else {
		articles, err := l.svcCtx.Repo.Articles.ListArticlesByAuthor(ctx, in.AuthorId, publishedStatus, limit, offset)
		if err != nil {
			l.Errorf("Failed to list author articles: %v", err)
			return nil, status.Error(codes.Internal, "failed to list author articles")
		}
		pbArticles = make([]*client.Article, len(articles))
		for i, a := range articles {
			pbArticles[i] = convertAuthorRowToPbArticle(a)
		}
	}

	if len(pbArticles) > 0 {
		articleIDs := make([]uuid.UUID, 0, len(pbArticles))
		for _, a := range pbArticles {
			if id, err := uuid.Parse(a.Id); err == nil {
				articleIDs = append(articleIDs, id)
			}
		}
		tagRows, err := l.svcCtx.Repo.Articles.GetTagsByArticleIDs(ctx, articleIDs)
		if err != nil {
			l.Errorf("failed to get article tags: %v", err)
		} else {
			tagMap := make(map[string][]string)
			for _, t := range tagRows {
				tagMap[t.ArticleID.String()] = append(tagMap[t.ArticleID.String()], t.Name)
			}
			for _, a := range pbArticles {
				a.Tags = tagMap[a.Id]
			}
		}
	}

	return &client.GetAuthorArticlesResponse{
		Articles: pbArticles,
	}, nil
}
