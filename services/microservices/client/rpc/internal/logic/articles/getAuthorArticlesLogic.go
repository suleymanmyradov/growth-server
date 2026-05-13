package articleslogic

import (
	"context"

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
		l.Errorf("Failed to list author articles: %v", err)
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
