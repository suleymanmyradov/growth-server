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

type DeleteArticleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteArticleLogic {
	return &DeleteArticleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteArticleLogic) DeleteArticle(in *client.DeleteArticleRequest) (*client.DeleteArticleResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "DeleteArticleLogic.DeleteArticle")
	defer span.End()
	articleID, err := uuid.Parse(in.ArticleId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid article id")
	}

	if err := l.svcCtx.Repo.Articles.DeleteArticle(ctx, articleID); err != nil {
		l.Errorf("delete article failed: %v", err)
		return nil, status.Error(codes.Internal, "delete article failed")
	}

	return &client.DeleteArticleResponse{Success: true}, nil
}
