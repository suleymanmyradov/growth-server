package logic

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/meilisearch/meilisearch-go"
	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/pb/search"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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
	_, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "SearchLogic.Search")
	defer span.End()

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

	filters, err := buildFilters(req)
	if err != nil {
		return nil, err
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
	counts := make(map[string]int32, len(result.Hits))

	results := make([]*search.SearchResult, 0, len(result.Hits))
	for _, hit := range result.Hits {
		doc, ok := hit.(map[string]any)
		if !ok {
			continue
		}

		entityType, _ := doc["type"].(string)
		title, _ := doc["title"].(string)
		description, _ := doc["description"].(string)
		url, _ := doc["url"].(string)

		// Build metadata (at most 2 keys)
		metadata := make(map[string]string, 2)
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

// buildFilters assembles the Meilisearch filter clauses for a search request.
// It is separated from Search so the authorization-critical logic can be unit
// tested without faking the whole meilisearch.IndexManager interface.
//
// The returned slice is AND-ed by Meilisearch, and element 0 is always the
// security clause.
func buildFilters(req *search.SearchRequest) ([]string, error) {
	// Build filters.
	//
	// Every value interpolated below is either validated against a closed set
	// or escaped by quoteFilterValue. Callers pass HTTP query parameters
	// straight through (see adminway's article list), so untrusted input must
	// never reach the filter expression unescaped. The security clause is
	// always element 0 and Meili ANDs the elements together, so it cannot be
	// widened by a later clause — but we do not rely on that alone.
	filters := make([]string, 0, 4)

	// Security filter: public docs OR the caller's own private docs.
	// The user id is interpolated, so it must be a well-formed UUID; callers
	// pass the authenticated principal's id and anything else is a bug
	// upstream, so we fail closed rather than degrade to a public-only search.
	userID := strings.TrimSpace(req.UserId)
	if userID != "" {
		if _, err := uuid.Parse(userID); err != nil {
			return nil, fmt.Errorf("search: invalid user id: %w", err)
		}
		filters = append(filters, `(visibility = "public" OR user_id = `+quoteFilterValue(userID)+`)`)
	} else {
		filters = append(filters, `visibility = "public"`)
	}

	// Type filter. Closed set — the indexer only ever writes these three.
	if len(req.Types) > 0 {
		typeFilters := make([]string, 0, len(req.Types))
		for _, t := range req.Types {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			if !allowedTypes[t] {
				return nil, fmt.Errorf("search: unsupported type %q", t)
			}
			typeFilters = append(typeFilters, `type = `+quoteFilterValue(t))
		}
		if len(typeFilters) > 0 {
			filters = append(filters, "("+strings.Join(typeFilters, " OR ")+")")
		}
	}

	// Category filter. Open set (domain categories are user/DB defined), so
	// this is escaped rather than allowlisted.
	if cat := strings.TrimSpace(req.Category); cat != "" {
		filters = append(filters, `category = `+quoteFilterValue(cat))
	}

	// Status filter (e.g. only published articles for admin search).
	// Closed set: only article docs carry a status.
	if st := strings.TrimSpace(req.Status); st != "" {
		if !allowedStatuses[st] {
			return nil, fmt.Errorf("search: unsupported status %q", st)
		}
		filters = append(filters, `status = `+quoteFilterValue(st))
	}

	return filters, nil
}

// allowedTypes is the closed set of `type` values the search-sync indexer
// writes to the catalog index (see search-sync repository GetArticle/GetGoal/
// GetHabit). A value outside this set can only be a caller bug or an
// injection attempt, so Search rejects it instead of silently matching
// nothing.
var allowedTypes = map[string]bool{
	"article": true,
	"goal":    true,
	"habit":   true,
}

// allowedStatuses mirrors the articles.status CHECK constraint
// (migration 027). Only article docs carry a status.
var allowedStatuses = map[string]bool{
	"draft":     true,
	"published": true,
}

// quoteFilterValue renders v as a double-quoted Meilisearch filter literal,
// escaping backslashes and quotes so the value cannot terminate its own
// string and inject filter syntax.
func quoteFilterValue(v string) string {
	var b strings.Builder
	b.Grow(len(v) + 2)
	b.WriteByte('"')
	for i := 0; i < len(v); i++ {
		if c := v[i]; c == '\\' || c == '"' {
			b.WriteByte('\\')
		}
		b.WriteByte(v[i])
	}
	b.WriteByte('"')
	return b.String()
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
