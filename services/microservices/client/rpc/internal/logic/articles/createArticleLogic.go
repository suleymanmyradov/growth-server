package articleslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateArticleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateArticleLogic {
	return &CreateArticleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateArticleLogic) CreateArticle(in *client.CreateArticleRequest) (*client.CreateArticleResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "CreateArticleLogic.CreateArticle")
	defer span.End()
	var categoryID uuid.NullUUID
	if in.CategoryId != "" {
		cid, err := uuid.Parse(in.CategoryId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid category id")
		}
		categoryID = uuid.NullUUID{UUID: cid, Valid: true}
	}

	var excerpt *string
	if in.Summary != "" {
		excerpt = &in.Summary
	}
	var imageUrl *string
	if in.CoverImage != "" {
		imageUrl = &in.CoverImage
	}

	articleStatus := in.Status
	if articleStatus == "" {
		articleStatus = "published"
	}

	row, err := l.svcCtx.Repo.Articles.CreateArticle(ctx, db.CreateArticleParams{
		Title:           in.Title,
		Excerpt:         excerpt,
		Content:         in.Content,
		CategoryID:      categoryID,
		ReadTimeMinutes: in.ReadTime,
		ImageUrl:        imageUrl,
		Author:          in.AuthorId,
		Status:          articleStatus,
	})
	if err != nil {
		l.Errorf("create article failed: %v", err)
		return nil, status.Error(codes.Internal, "create article failed")
	}

	pb := &client.Article{
		Id:          row.ID.String(),
		Title:       row.Title,
		Content:     row.Content,
		AuthorId:    row.Author,
		ReadTime:    row.ReadTime,
		PublishedAt: row.PublishedAt.Time.Unix(),
		CreatedAt:   row.CreatedAt.Time.Unix(),
		UpdatedAt:   row.UpdatedAt.Time.Unix(),
		Status:      row.Status,
	}
	if row.Excerpt != nil {
		pb.Summary = *row.Excerpt
	}
	if row.ImageUrl != nil {
		pb.CoverImage = *row.ImageUrl
	}
	if categoryID.Valid {
		cat, err := l.svcCtx.Repo.Categories.GetCategoryByID(ctx, categoryID.UUID)
		if err == nil {
			pb.Category = &client.ArticleCategory{
				Id:   cat.ID.String(),
				Name: cat.Name,
				Slug: cat.Slug,
			}
		}
	}

	if len(in.Tags) > 0 {
		tagSlugs := slugifyTags(in.Tags)
		if _, err := l.svcCtx.Repo.Articles.UpsertTags(ctx, in.Tags, tagSlugs); err != nil {
			l.Errorf("upsert tags failed: %v", err)
		}
		if err := l.svcCtx.Repo.Articles.LinkArticleTags(ctx, row.ID, in.Tags); err != nil {
			l.Errorf("link article tags failed: %v", err)
		}
		pb.Tags = in.Tags
	}
	if pb.Tags == nil {
		pb.Tags = []string{}
	}

	return &client.CreateArticleResponse{Article: pb}, nil
}
