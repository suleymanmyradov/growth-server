package articleslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	searchservice "github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/searchservice"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SearchArticlesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSearchArticlesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchArticlesLogic {
	return &SearchArticlesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SearchArticles performs article full-text search via the Meilisearch-backed
// search microservice, then hydrates the matching article IDs into full Article
// protos (with category, like count and tags) from the database. Results are
// reordered to preserve the relevance order returned by the search service.
func (l *SearchArticlesLogic) SearchArticles(in *client.SearchArticlesRequest) (*client.SearchArticlesResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "SearchArticlesLogic.SearchArticles")
	defer span.End()
	if in.Query == "" {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	limit := int32(20)
	offset := int32(0)
	if in.Limit > 0 {
		limit = in.Limit
	}
	if in.Offset > 0 {
		offset = in.Offset
	}

	// 1. Match article IDs in Meilisearch (filtered by status when requested).
	searchResp, err := l.svcCtx.SearchRpc.Search(ctx, &searchservice.SearchRequest{
		Query:  in.Query,
		Types:  []string{"article"},
		Status: in.Status,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		l.Errorf("search rpc failed: %v", err)
		return nil, status.Error(codes.Internal, "search articles failed")
	}

	// 2. Nothing matched: short-circuit with an empty page.
	if len(searchResp.Results) == 0 {
		return &client.SearchArticlesResponse{
			Articles:   []*client.Article{},
			TotalCount: searchResp.Total,
		}, nil
	}

	// 3. Parse the matched entity IDs and hydrate full article rows from the DB.
	ids := make([]uuid.UUID, 0, len(searchResp.Results))
	for _, r := range searchResp.Results {
		if id, err := uuid.Parse(r.Id); err == nil {
			ids = append(ids, id)
		}
	}

	rows, err := l.svcCtx.Repo.Articles.GetArticlesByIDs(ctx, ids)
	if err != nil {
		l.Errorf("hydrate articles by ids failed: %v", err)
		return nil, status.Error(codes.Internal, "search articles failed")
	}

	byID := make(map[string]db.GetArticlesByIDsRow, len(rows))
	for _, r := range rows {
		byID[r.ID.String()] = r
	}

	// 4. Rebuild the article list in the relevance order from the search service.
	pbArticles := make([]*client.Article, 0, len(searchResp.Results))
	for _, r := range searchResp.Results {
		row, ok := byID[r.Id]
		if !ok {
			// Stale index hit: the article was deleted/changed after matching.
			// Skip it rather than returning a partial/empty article.
			continue
		}
		pbArticles = append(pbArticles, convertGetArticlesByIDsRowToPbArticle(row))
	}

	// 5. Attach tags in bulk (same pattern as ListArticles).
	if len(pbArticles) > 0 {
		articleIDs := make([]uuid.UUID, 0, len(pbArticles))
		for _, a := range pbArticles {
			if id, err := uuid.Parse(a.Id); err == nil {
				articleIDs = append(articleIDs, id)
			}
		}
		tagRows, err := l.svcCtx.Repo.Articles.GetTagsByArticleIDs(ctx, articleIDs)
		if err != nil {
			l.Errorf("failed to get article tags: %v", err)
		} else {
			tagMap := make(map[string][]string)
			for _, t := range tagRows {
				tagMap[t.ArticleID.String()] = append(tagMap[t.ArticleID.String()], t.Name)
			}
			for _, a := range pbArticles {
				tags := tagMap[a.Id]
				if tags == nil {
					tags = []string{}
				}
				a.Tags = tags
			}
		}
	}

	return &client.SearchArticlesResponse{
		Articles:   pbArticles,
		TotalCount: searchResp.Total,
	}, nil
}
