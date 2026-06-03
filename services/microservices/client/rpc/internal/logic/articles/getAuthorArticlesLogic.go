package articleslogic

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/google/uuid"
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

	userID, userErr := uuid.Parse(in.UserId)
	hasUser := in.UserId != "" && userErr == nil

	var pbArticles []*client.Article
	if hasUser {
		articles, err := l.svcCtx.Repo.Articles.ListArticlesByAuthorWithSaved(l.ctx, in.AuthorId, limit, offset, userID)
		if err != nil {
			l.Errorf("Failed to list author articles with saved: %v", err)
			return nil, status.Error(codes.Internal, "failed to list author articles")
		}
		pbArticles = make([]*client.Article, len(articles))
		for i, a := range articles {
			pbArticles[i] = convertAuthorWithSavedRowToPbArticle(a)
		}
	} else {
		articles, err := l.svcCtx.Repo.Articles.ListArticlesByAuthor(l.ctx, in.AuthorId, limit, offset)
		if err != nil {
			l.Errorf("Failed to list author articles: %v", err)
			return nil, status.Error(codes.Internal, "failed to list author articles")
		}
		pbArticles = make([]*client.Article, len(articles))
		for i, a := range articles {
			pbArticles[i] = convertAuthorRowToPbArticle(a)
		}
	}

	return &client.GetAuthorArticlesResponse{
		Articles: pbArticles,
	}, nil
}
