package articleslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetArticleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetArticleLogic {
	return &GetArticleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetArticleLogic) GetArticle(in *client.GetArticleRequest) (*client.GetArticleResponse, error) {
	articleID, err := uuid.Parse(in.ArticleId)
	if err != nil {
		l.Logger.Errorf("Invalid article ID: %v", err)
		return nil, err
	}

	article, err := l.svcCtx.Repo.Articles.GetArticle(l.ctx, articleID)
	if err != nil {
		l.Logger.Errorf("Failed to get article: %v", err)
		return nil, err
	}

	return &client.GetArticleResponse{
		Article: convertGetRowToPbArticle(article),
	}, nil
}

func convertGetRowToPbArticle(a db.GetArticleRow) *client.Article {
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
