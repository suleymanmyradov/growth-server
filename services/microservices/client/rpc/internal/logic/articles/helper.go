package articleslogic

import (
	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

func convertGetRowToPbArticle(a db.GetArticleRow) *client.Article {
	pb := &client.Article{
		Id:          a.ID.String(),
		Title:       a.Title,
		Content:     a.Content,
		AuthorId:    a.Author,
		ReadTime:    a.ReadTime,
		PublishedAt: a.PublishedAt.Unix(),
		CreatedAt:   a.CreatedAt.Unix(),
		UpdatedAt:   a.UpdatedAt.Unix(),
	}
	if a.Excerpt.Valid {
		pb.Summary = a.Excerpt.String
	}
	if a.ImageUrl.Valid {
		pb.CoverImage = a.ImageUrl.String
	}
	// Map category from joined data
	if a.CategoryID.Valid && a.CategoryID.UUID != uuid.Nil {
		pb.Category = &client.ArticleCategory{
			Id:   a.CategoryID.UUID.String(),
			Name: a.CategoryName.String,
			Slug: a.CategorySlug.String,
		}
	}
	return pb
}

func convertAuthorRowToPbArticle(a db.ListArticlesByAuthorRow) *client.Article {
	pb := &client.Article{
		Id:          a.ID.String(),
		Title:       a.Title,
		Content:     a.Content,
		AuthorId:    a.Author,
		ReadTime:    a.ReadTime,
		PublishedAt: a.PublishedAt.Unix(),
		CreatedAt:   a.CreatedAt.Unix(),
		UpdatedAt:   a.UpdatedAt.Unix(),
	}
	if a.Excerpt.Valid {
		pb.Summary = a.Excerpt.String
	}
	if a.ImageUrl.Valid {
		pb.CoverImage = a.ImageUrl.String
	}
	// Map category from joined data
	if a.CategoryID.Valid && a.CategoryID.UUID != uuid.Nil {
		pb.Category = &client.ArticleCategory{
			Id:   a.CategoryID.UUID.String(),
			Name: a.CategoryName.String,
			Slug: a.CategorySlug.String,
		}
	}
	return pb
}

func convertListRowToPbArticle(a db.ListArticlesRow) *client.Article {
	pb := &client.Article{
		Id:          a.ID.String(),
		Title:       a.Title,
		Content:     a.Content,
		AuthorId:    a.Author,
		ReadTime:    a.ReadTime,
		PublishedAt: a.PublishedAt.Unix(),
		CreatedAt:   a.CreatedAt.Unix(),
		UpdatedAt:   a.UpdatedAt.Unix(),
	}
	if a.Excerpt.Valid {
		pb.Summary = a.Excerpt.String
	}
	if a.ImageUrl.Valid {
		pb.CoverImage = a.ImageUrl.String
	}
	// Map category from joined data
	if a.CategoryID.Valid && a.CategoryID.UUID != uuid.Nil {
		pb.Category = &client.ArticleCategory{
			Id:   a.CategoryID.UUID.String(),
			Name: a.CategoryName.String,
			Slug: a.CategorySlug.String,
		}
	}
	return pb
}

func convertCategorySlugRowToPbArticle(a db.ListArticlesByCategorySlugRow) *client.Article {
	pb := &client.Article{
		Id:          a.ID.String(),
		Title:       a.Title,
		Content:     a.Content,
		AuthorId:    a.Author,
		ReadTime:    a.ReadTime,
		PublishedAt: a.PublishedAt.Unix(),
		CreatedAt:   a.CreatedAt.Unix(),
		UpdatedAt:   a.UpdatedAt.Unix(),
	}
	if a.Excerpt.Valid {
		pb.Summary = a.Excerpt.String
	}
	if a.ImageUrl.Valid {
		pb.CoverImage = a.ImageUrl.String
	}
	// Map category from joined data (non-nullable in this query)
	if a.CategoryID != uuid.Nil {
		pb.Category = &client.ArticleCategory{
			Id:   a.CategoryID.String(),
			Name: a.CategoryName,
			Slug: a.CategorySlug,
		}
	}
	return pb
}
