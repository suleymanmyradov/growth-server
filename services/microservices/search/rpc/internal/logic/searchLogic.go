package logic

import (
	"context"
	"fmt"
	"strings"

	"github.com/meilisearch/meilisearch-go"
	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/pb/search"
	"github.com/zeromicro/go-zero/core/logx"
)

type SearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchLogic {
	return &SearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SearchLogic) Search(req *search.SearchRequest) (*search.SearchResponse, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return &search.SearchResponse{}, nil
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	// Build filters
	var filters []string

	// Security filter: public articles OR user's own private docs
	userID := strings.TrimSpace(req.UserId)
	if userID != "" {
		filters = append(filters, fmt.Sprintf(`(visibility = "public" OR user_id = "%s")`, userID))
	} else {
		filters = append(filters, `visibility = "public"`)
	}

	// Type filter
	if len(req.Types) > 0 {
		var typeFilters []string
		for _, t := range req.Types {
			typeFilters = append(typeFilters, fmt.Sprintf(`type = "%s"`, t))
		}
		filters = append(filters, "("+strings.Join(typeFilters, " OR ")+")")
	}

	// Category filter
	if strings.TrimSpace(req.Category) != "" {
		filters = append(filters, fmt.Sprintf(`category = "%s"`, req.Category))
	}

	searchReq := &meilisearch.SearchRequest{
		Offset: int64(offset),
		Limit:  int64(limit),
		Filter: filters,
	}

	result, err := l.svcCtx.Index.Search(query, searchReq)
	if err != nil {
		return nil, fmt.Errorf("meilisearch search: %w", err)
	}

	// Aggregate counts by type
	counts := make(map[string]int32)

	var results []*search.SearchResult
	for _, hit := range result.Hits {
		doc, ok := hit.(map[string]any)
		if !ok {
			continue
		}

		id, _ := doc["id"].(string)
		_ = id

		entityType, _ := doc["type"].(string)
		title, _ := doc["title"].(string)
		description, _ := doc["description"].(string)
		url, _ := doc["url"].(string)

		// Build metadata
		metadata := make(map[string]string)
		if v, ok := doc["category"].(string); ok && v != "" {
			metadata["category"] = v
		}
		if v, ok := doc["author"].(string); ok && v != "" {
			metadata["author"] = v
		}

		res := &search.SearchResult{
			Id:          getString(doc, "entity_id"),
			Type:        entityType,
			Title:       title,
			Description: description,
			Url:         url,
			Metadata:    metadata,
			Score:       0,
		}

		results = append(results, res)
		counts[entityType]++
	}

	// Fill approximate scores based on hit order (Meilisearch relevance)
	for i := range results {
		if len(results) > 1 {
			results[i].Score = float32(len(results)-i) / float32(len(results))
		} else {
			results[i].Score = 1.0
		}
	}

	total := int32(result.EstimatedTotalHits)
	if total == 0 {
		total = int32(result.TotalHits)
	}

	return &search.SearchResponse{
		Results: results,
		Total:   total,
		Counts:  counts,
	}, nil
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	if v, ok := m[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}
