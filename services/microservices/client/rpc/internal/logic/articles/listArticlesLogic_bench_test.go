package articleslogic

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

func makeListArticlesRows(n int) []db.ListArticlesRow {
	rows := make([]db.ListArticlesRow, n)
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	for i := range rows {
		rows[i] = db.ListArticlesRow{
			ID:          uuid.New(),
			Title:       "Article " + string(rune('A'+i%26)),
			Content:     "content",
			Author:      uuid.New().String(),
			ReadTime:    5,
			PublishedAt: now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
	}
	return rows
}

func BenchmarkConvertListRowToPbArticle(b *testing.B) {
	rows := makeListArticlesRows(50)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out := make([]*client.Article, len(rows))
		for j, r := range rows {
			out[j] = convertListRowToPbArticle(r)
		}
		_ = out
	}
}
