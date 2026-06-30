package logic

import (
	"testing"

	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/pb/search"
)

func makeSearchHits(n int) []any {
	hits := make([]any, n)
	for i := range hits {
		hits[i] = map[string]any{
			"id":          i,
			"entity_id":   "ent-" + string(rune('0'+i%10)),
			"type":        "article",
			"title":       "Title " + string(rune('A'+i%26)),
			"description": "Desc",
			"url":         "/url",
			"category":    "cat",
			"author":      "auth",
		}
	}
	return hits
}

func BenchmarkBuildResults(b *testing.B) {
	hits := makeSearchHits(20)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := make([]*search.SearchResult, 0, len(hits))
		counts := make(map[string]int32, len(hits))
		for j, hit := range hits {
			doc, ok := hit.(map[string]any)
			if !ok {
				continue
			}
			entityType, _ := doc["type"].(string)
			title, _ := doc["title"].(string)
			description, _ := doc["description"].(string)
			url, _ := doc["url"].(string)

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
			_ = j
		}
		for j := range results {
			if len(results) > 1 {
				results[j].Score = float32(len(results)-j) / float32(len(results))
			} else {
				results[j].Score = 1.0
			}
		}
		_ = results
		_ = counts
	}
}
