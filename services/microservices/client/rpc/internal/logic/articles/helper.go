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
		PublishedAt: a.PublishedAt.Time.Unix(),
		CreatedAt:   a.CreatedAt.Time.Unix(),
		UpdatedAt:   a.UpdatedAt.Time.Unix(),
	}
	if a.Excerpt != nil {
		pb.Summary = *a.Excerpt
	}
	if a.ImageUrl != nil {
		pb.CoverImage = *a.ImageUrl
	}
	// Map category from joined data
	if a.CategoryID.Valid && a.CategoryID.UUID != uuid.Nil {
		categoryName := ""
		if a.CategoryName != nil {
			categoryName = *a.CategoryName
		}
		categorySlug := ""
		if a.CategorySlug != nil {
			categorySlug = *a.CategorySlug
		}
		pb.Category = &client.ArticleCategory{
			Id:   a.CategoryID.UUID.String(),
			Name: categoryName,
			Slug: categorySlug,
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
		PublishedAt: a.PublishedAt.Time.Unix(),
		CreatedAt:   a.CreatedAt.Time.Unix(),
		UpdatedAt:   a.UpdatedAt.Time.Unix(),
	}
	if a.Excerpt != nil {
		pb.Summary = *a.Excerpt
	}
	if a.ImageUrl != nil {
		pb.CoverImage = *a.ImageUrl
	}
	// Map category from joined data
	if a.CategoryID.Valid && a.CategoryID.UUID != uuid.Nil {
		categoryName := ""
		if a.CategoryName != nil {
			categoryName = *a.CategoryName
		}
		categorySlug := ""
		if a.CategorySlug != nil {
			categorySlug = *a.CategorySlug
		}
		pb.Category = &client.ArticleCategory{
			Id:   a.CategoryID.UUID.String(),
			Name: categoryName,
			Slug: categorySlug,
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
		PublishedAt: a.PublishedAt.Time.Unix(),
		CreatedAt:   a.CreatedAt.Time.Unix(),
		UpdatedAt:   a.UpdatedAt.Time.Unix(),
	}
	if a.Excerpt != nil {
		pb.Summary = *a.Excerpt
	}
	if a.ImageUrl != nil {
		pb.CoverImage = *a.ImageUrl
	}
	// Map category from joined data
	if a.CategoryID.Valid && a.CategoryID.UUID != uuid.Nil {
		categoryName := ""
		if a.CategoryName != nil {
			categoryName = *a.CategoryName
		}
		categorySlug := ""
		if a.CategorySlug != nil {
			categorySlug = *a.CategorySlug
		}
		pb.Category = &client.ArticleCategory{
			Id:   a.CategoryID.UUID.String(),
			Name: categoryName,
			Slug: categorySlug,
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
		PublishedAt: a.PublishedAt.Time.Unix(),
		CreatedAt:   a.CreatedAt.Time.Unix(),
		UpdatedAt:   a.UpdatedAt.Time.Unix(),
	}
	if a.Excerpt != nil {
		pb.Summary = *a.Excerpt
	}
	if a.ImageUrl != nil {
		pb.CoverImage = *a.ImageUrl
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
