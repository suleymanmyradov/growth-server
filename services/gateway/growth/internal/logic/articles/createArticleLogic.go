package articles

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientarticles "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/articles"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/fileManagerClient"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateArticleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateArticleLogic {
	return &CreateArticleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateArticleLogic) CreateArticle(req *types.CreateArticleRequest, coverImageData []byte, coverImageFilename, coverImageContentType string) (resp *types.ArticleResponse, err error) {
	if req.Title == "" {
		return nil, status.Error(codes.InvalidArgument, "title is required")
	}

	coverImage := ""
	if len(coverImageData) > 0 {
		uploadResp, err := l.svcCtx.FileManagerRpc.UploadFile(l.ctx, &fileManagerClient.UploadFileRequest{
			Data:        coverImageData,
			Filename:    coverImageFilename,
			ContentType: coverImageContentType,
			Folder:      "articles",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to upload cover image: %w", err)
		}
		coverImage = uploadResp.Url
	}

	rpcReq := &clientarticles.CreateArticleRequest{
		Title:      req.Title,
		Content:    req.Content,
		Summary:    req.Summary,
		AuthorId:   req.AuthorId,
		CoverImage: coverImage,
		Tags:       req.Tags,
		ReadTime:   int32(req.ReadTime),
		CategoryId: req.CategoryId,
	}

	rpcResp, err := l.svcCtx.ArticlesRpc.CreateArticle(l.ctx, rpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create article via rpc: %w", err)
	}

	if rpcResp.Article == nil {
		return nil, status.Error(codes.Internal, "article creation returned nil")
	}

	rpcArticle := rpcResp.Article
	category := l.mapCategory(rpcArticle.Category)

	article := types.Article{
		Id:          rpcArticle.Id,
		Title:       rpcArticle.Title,
		Excerpt:     rpcArticle.Summary,
		Content:     rpcArticle.Content,
		Category:    category,
		ReadTime:    int(rpcArticle.ReadTime),
		ImageUrl:    rpcArticle.CoverImage,
		Author:      rpcArticle.AuthorId,
		PublishedAt: formatTime(rpcArticle.PublishedAt),
		CreatedAt:   formatTime(rpcArticle.CreatedAt),
		UpdatedAt:   formatTime(rpcArticle.UpdatedAt),
		IsSaved:     rpcArticle.IsSaved,
		Tags:        rpcArticle.Tags,
	}

	return &types.ArticleResponse{
		Data: article,
	}, nil
}

func (l *CreateArticleLogic) mapCategory(rpcCategory *client.ArticleCategory) *types.ArticleCategory {
	if rpcCategory == nil {
		return nil
	}
	return &types.ArticleCategory{
		Id:   rpcCategory.Id,
		Name: rpcCategory.Name,
		Slug: rpcCategory.Slug,
	}
}
