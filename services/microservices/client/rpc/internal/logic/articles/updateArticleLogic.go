package articleslogic

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdateArticleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateArticleLogic {
	return &UpdateArticleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateArticleLogic) UpdateArticle(in *client.UpdateArticleRequest) (*client.UpdateArticleResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "UpdateArticleLogic.UpdateArticle")
	defer span.End()
	articleID, err := uuid.Parse(in.ArticleId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid article id")
	}

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

	// Empty status means "keep current" (enforced by the UPDATE query); only
	// draft and published are valid explicit transitions.
	switch in.Status {
	case "", "draft", "published":
	default:
		return nil, status.Error(codes.InvalidArgument, "invalid article status")
	}

	// Article update and tag rewrite must be atomic — running them on the pool
	// leaves the article with zero tags if the link step fails partway.
	var row db.UpdateArticleRow
	err = l.svcCtx.RunInTx(ctx, "", func(txRepo *repository.Repository) error {
		var uErr error
		row, uErr = txRepo.Articles.UpdateArticle(ctx, db.UpdateArticleParams{
			ID:              articleID,
			Title:           in.Title,
			Excerpt:         excerpt,
			Content:         in.Content,
			CategoryID:      categoryID,
			ReadTimeMinutes: in.ReadTime,
			ImageUrl:        imageUrl,
			Author:          in.AuthorId,
			Status:          in.Status,
		})
		if uErr != nil {
			return fmt.Errorf("update article: %w", uErr)
		}

		// Tags are only rewritten when the request carries a non-empty list —
		// an omitted list must preserve existing tags (there is no wire-level
		// way to distinguish "leave alone" from "clear all" today).
		if len(in.Tags) > 0 {
			if dErr := txRepo.Articles.DeleteArticleTags(ctx, articleID); dErr != nil {
				return fmt.Errorf("delete article tags: %w", dErr)
			}
			tagSlugs := slugifyTags(in.Tags)
			if _, uErr := txRepo.Articles.UpsertTags(ctx, in.Tags, tagSlugs); uErr != nil {
				return fmt.Errorf("upsert tags: %w", uErr)
			}
			if lErr := txRepo.Articles.LinkArticleTags(ctx, articleID, in.Tags); lErr != nil {
				return fmt.Errorf("link article tags: %w", lErr)
			}
		}
		return nil
	})
	if err != nil {
		l.Errorf("update article failed: %v", err)
		return nil, status.Error(codes.Internal, "update article failed")
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
	if row.CategoryID.Valid {
		cat, err := l.svcCtx.Repo.Categories.GetCategoryByID(ctx, row.CategoryID.UUID)
		if err == nil {
			pb.Category = &client.ArticleCategory{
				Id:   cat.ID.String(),
				Name: cat.Name,
				Slug: cat.Slug,
			}
		}
	}

	if len(in.Tags) > 0 {
		pb.Tags = in.Tags
	} else {
		// Tags were preserved server-side — echo the persisted set back.
		tagRows, err := l.svcCtx.Repo.Articles.GetTagsByArticleIDs(ctx, []uuid.UUID{articleID})
		if err == nil {
			pb.Tags = make([]string, 0, len(tagRows))
			for _, t := range tagRows {
				pb.Tags = append(pb.Tags, t.Name)
			}
		}
	}
	if pb.Tags == nil {
		pb.Tags = []string{}
	}

	return &client.UpdateArticleResponse{Article: pb}, nil
}
