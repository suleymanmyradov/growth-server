package articleslogic

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
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetArticleLogic.GetArticle")
	defer span.End()
	if in == nil || in.ArticleId == "" {
		return nil, status.Error(codes.InvalidArgument, "article ID is required")
	}

	articleID, err := uuid.Parse(in.ArticleId)
	if err != nil {
		l.Errorf("failed to parse article ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid article ID")
	}

	// Parse user ID if provided
	userID, userErr := uuid.Parse(in.UserId)
	hasUser := in.UserId != "" && userErr == nil

	var pbArticle *client.Article
	if hasUser {
		article, err := l.svcCtx.Repo.Articles.GetArticleByIDWithSaved(ctx, articleID, userID, in.Status)
		if err != nil {
			l.Errorf("failed to get article with saved: %v", err)
			return nil, status.Error(codes.NotFound, "article not found")
		}
		pbArticle = convertGetWithSavedRowToPbArticle(article)
	} else {
		article, err := l.svcCtx.Repo.Articles.GetArticleByID(ctx, articleID, in.Status)
		if err != nil {
			l.Errorf("failed to get article: %v", err)
			return nil, status.Error(codes.NotFound, "article not found")
		}
		pbArticle = convertGetRowToPbArticle(article)
	}

	tagRows, err := l.svcCtx.Repo.Articles.GetTagsByArticleIDs(ctx, []uuid.UUID{articleID})
	if err != nil {
		l.Errorf("failed to get article tags: %v", err)
	} else {
		pbArticle.Tags = make([]string, 0, len(tagRows))
		for _, t := range tagRows {
			pbArticle.Tags = append(pbArticle.Tags, t.Name)
		}
	}

	return &client.GetArticleResponse{
		Article: pbArticle,
	}, nil
}
