package articleslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"github.com/zeromicro/go-zero/core/logx"
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
	articleID, err := uuid.Parse(in.ArticleId)
	if err != nil {
		l.Logger.Errorf("Invalid article ID: %v", err)
		return nil, err
	}

	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Logger.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	_, err = l.svcCtx.Repo.Articles.CreateArticleShare(l.ctx, db.CreateArticleShareParams{
		ArticleID: articleID,
		UserID:    userID,
		Platform:  in.Platform,
	})
	if err != nil {
		l.Logger.Errorf("Failed to create article share: %v", err)
		return nil, err
	}

	return &client.ShareArticleResponse{
		Success: true,
	}, nil
}
