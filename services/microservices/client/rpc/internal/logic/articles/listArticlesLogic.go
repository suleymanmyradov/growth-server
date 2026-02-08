package articleslogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListArticlesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListArticlesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListArticlesLogic {
	return &ListArticlesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListArticlesLogic) ListArticles(in *client.ListArticlesRequest) (*client.ListArticlesResponse, error) {
	limit := int32(20)
	offset := int32(0)
	if in.Limit > 0 {
		limit = in.Limit
	}
	if in.Offset > 0 {
		offset = in.Offset
	}

	articles, err := l.svcCtx.Repo.Articles.ListArticles(l.ctx, limit, offset)
	if err != nil {
		l.Logger.Errorf("Failed to list articles: %v", err)
		return nil, err
	}

	totalCount, err := l.svcCtx.Repo.Articles.CountArticles(l.ctx)
	if err != nil {
		l.Logger.Errorf("Failed to count articles: %v", err)
	}

	var pbArticles []*client.Article
	for _, a := range articles {
		pbArticles = append(pbArticles, convertListRowToPbArticle(a))
	}

	return &client.ListArticlesResponse{
		Articles:   pbArticles,
		TotalCount: int32(totalCount),
	}, nil
}

func convertListRowToPbArticle(a db.ListArticlesRow) *client.Article {
	pb := &client.Article{
		Id:          a.ID.String(),
		Title:       a.Title,
		Content:     a.Content,
		AuthorId:    a.Author,
		ReadTime:    a.ReadTime,
		PublishedAt: a.PublishedAt.Time.Unix(),
	}
	if a.Excerpt.Valid {
		pb.Summary = a.Excerpt.String
	}
	if a.ImageUrl.Valid {
		pb.CoverImage = a.ImageUrl.String
	}
	return pb
}
