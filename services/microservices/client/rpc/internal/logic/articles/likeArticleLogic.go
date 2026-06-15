package articleslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LikeArticleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLikeArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LikeArticleLogic {
	return &LikeArticleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LikeArticleLogic) LikeArticle(in *client.LikeArticleRequest) (*client.LikeArticleResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "LikeArticleLogic.LikeArticle")
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

	// Check if already liked
	isLiked, err := l.svcCtx.Repo.Articles.IsArticleLikedByUser(ctx, articleID, userID)
	if err != nil {
		l.Errorf("Failed to check if article is liked: %v", err)
		return nil, status.Error(codes.Internal, "failed to check like status")
	}

	if isLiked {
		// Unlike: delete the like
		err = l.svcCtx.Repo.Articles.DeleteArticleLike(ctx, articleID, userID)
		if err != nil {
			l.Errorf("Failed to delete article like: %v", err)
			return nil, status.Error(codes.Internal, "failed to unlike article")
		}
		l.Infof("User %s unliked article %s", userID, articleID)
	} else {
		// Like: create the like
		_, err = l.svcCtx.Repo.Articles.CreateArticleLike(ctx, articleID, userID)
		if err != nil {
			l.Errorf("Failed to create article like: %v", err)
			return nil, status.Error(codes.Internal, "failed to like article")
		}
		l.Infof("User %s liked article %s", userID, articleID)
	}

	// Get the updated count
	count, err := l.svcCtx.Repo.Articles.CountArticleLikes(ctx, articleID)
	if err != nil {
		l.Errorf("Failed to count article likes: %v", err)
		return nil, status.Error(codes.Internal, "failed to count likes")
	}

	return &client.LikeArticleResponse{
		Success:      true,
		NewLikeCount: int32(count),
		IsLiked:      !isLiked,
	}, nil
}
