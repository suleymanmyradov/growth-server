package articleslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
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
	if in == nil || in.ArticleId == "" {
		return nil, status.Error(codes.InvalidArgument, "article ID is required")
	}

	articleID, err := uuid.Parse(in.ArticleId)
	if err != nil {
		l.Errorf("failed to parse article ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid article ID")
	}

	article, err := l.svcCtx.Repo.Articles.GetArticleByID(l.ctx, articleID)
	if err != nil {
		l.Errorf("failed to get article: %v", err)
		return nil, status.Error(codes.NotFound, "article not found")
	}

	return &client.GetArticleResponse{
		Article: convertGetRowToPbArticle(article),
	}, nil
}
