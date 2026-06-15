package articleslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ShareArticleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewShareArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ShareArticleLogic {
	return &ShareArticleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ShareArticleLogic) ShareArticle(in *client.ShareArticleRequest) (*client.ShareArticleResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ShareArticleLogic.ShareArticle")
	defer span.End()
	articleID, err := uuid.Parse(in.ArticleId)
	if err != nil {
		l.Errorf("Invalid article ID: %v", err)
return nil, status.Error(codes.Internal, "invalid article id")
	}

	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
return nil, status.Error(codes.Internal, "invalid user id")
	}

	_, err = l.svcCtx.Repo.Articles.CreateArticleShare(ctx, articleID, userID, in.Platform)
	if err != nil {
		l.Errorf("Failed to create article share: %v", err)
return nil, status.Error(codes.Internal, "failed to create article share")
	}

	return &client.ShareArticleResponse{
		Success: true,
	}, nil
}
