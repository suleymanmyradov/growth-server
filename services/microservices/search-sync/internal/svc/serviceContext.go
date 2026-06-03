package svc

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/meilisearch/meilisearch-go"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/services/microservices/search-sync/internal/config"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceContext struct {
	Config config.Config
	Pool   *pgxpool.Pool
	Meili  meilisearch.ServiceManager
	Index  meilisearch.IndexManager
}

func NewServiceContext(c config.Config) *ServiceContext {
	pool := postgres.MustOpenPool(
		c.Postgres.Datasource,
		c.Postgres.MaxOpenConns,
		c.Postgres.MaxIdleConns,
		c.Postgres.ConnMaxLifetime,
	)

	client := meilisearch.New(c.Meili.Host, meilisearch.WithAPIKey(c.Meili.APIKey))

	// Ensure index exists
	index := client.Index(c.Meili.Index)
	_, err := client.GetIndex(c.Meili.Index)
	if err != nil {
		_, err = client.CreateIndex(&meilisearch.IndexConfig{
			Uid:        c.Meili.Index,
			PrimaryKey: "id",
		})
		if err != nil {
			logx.Must(fmt.Errorf("create meilisearch index: %w", err))
		}
	}

	// Apply index settings
	_, err = index.UpdateSearchableAttributes(&[]string{
		"title",
		"description",
		"content",
		"category",
		"author",
	})
	if err != nil {
		logx.Must(fmt.Errorf("update searchable attributes: %w", err))
	}

	_, err = index.UpdateFilterableAttributes(&[]string{
		"type",
		"user_id",
		"visibility",
		"category",
		"category_slug",
	})
	if err != nil {
		logx.Must(fmt.Errorf("update filterable attributes: %w", err))
	}

	_, err = index.UpdateSortableAttributes(&[]string{
		"created_at",
		"updated_at",
	})
	if err != nil {
		logx.Must(fmt.Errorf("update sortable attributes: %w", err))
	}

	_, err = index.UpdateRankingRules(&[]string{
		"words",
		"typo",
		"proximity",
		"attribute",
		"sort",
		"exactness",
	})
	if err != nil {
		logx.Must(fmt.Errorf("update ranking rules: %w", err))
	}

	return &ServiceContext{
		Config: c,
		Pool:   pool,
		Meili:  client,
		Index:  index,
	}
}

func (s *ServiceContext) Close() {
	if s.Pool != nil {
		s.Pool.Close()
	}
}
