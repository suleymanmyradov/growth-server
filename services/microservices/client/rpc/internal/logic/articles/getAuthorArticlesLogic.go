package articleslogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"github.com/zeromicro/go-zero/core/logx"
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
	limit := int32(20)
	offset := int32(0)
	if in.Limit > 0 {
		limit = in.Limit
	}
	if in.Offset > 0 {
		offset = in.Offset
	}

	articles, err := l.svcCtx.Repo.Articles.ListArticlesByAuthor(l.ctx, in.AuthorId, limit, offset)
	if err != nil {
		l.Logger.Errorf("Failed to list author articles: %v", err)
		return nil, err
	}

	var pbArticles []*client.Article
	for _, a := range articles {
		pbArticles = append(pbArticles, convertAuthorRowToPbArticle(a))
	}

	return &client.GetAuthorArticlesResponse{
		Articles: pbArticles,
	}, nil
}

func convertAuthorRowToPbArticle(a db.ListArticlesByAuthorRow) *client.Article {
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
