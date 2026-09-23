package articleslogic

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

	var isLiked bool
	if in.Liked != nil {
		// Desired-state semantics: the call is idempotent — repeating a request
		// (e.g. a mobile retry after a timeout) converges to the same state
		// instead of inverting it.
		if *in.Liked {
			if _, err := l.svcCtx.Repo.Articles.CreateArticleLike(ctx, articleID, userID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				l.Errorf("Failed to create article like: %v", err)
				return nil, status.Error(codes.Internal, "failed to like article")
			}
		} else {
			if err := l.svcCtx.Repo.Articles.DeleteArticleLike(ctx, articleID, userID); err != nil {
				l.Errorf("Failed to delete article like: %v", err)
				return nil, status.Error(codes.Internal, "failed to unlike article")
			}
		}
		isLiked = *in.Liked
	} else {
		// Legacy toggle path: flip atomically in one statement so concurrent
		// taps can't race a read-then-write.
		isLiked, err = l.svcCtx.Repo.Articles.ToggleArticleLike(ctx, articleID, userID)
		if err != nil {
			l.Errorf("Failed to toggle article like: %v", err)
			return nil, status.Error(codes.Internal, "failed to toggle like")
		}
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
		IsLiked:      isLiked,
	}, nil
}
