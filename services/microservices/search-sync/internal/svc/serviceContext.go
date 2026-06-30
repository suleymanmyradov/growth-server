package svc

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/meilisearch/meilisearch-go"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/services/microservices/search-sync/internal/config"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceContext struct {
	Config      config.Config
	Pool        *pgxpool.Pool
	Meili       meilisearch.ServiceManager
	Index       meilisearch.IndexManager
	MemoryIndex meilisearch.IndexManager
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

	// Private per-user memory index for the AI coach. This is intentionally a
	// SEPARATE index from the public catalog: every retrieval query filters on
	// user_id, so personal free-text never crosses users and never leaks into
	// GET /api/v1/search. It is optional; if MemoryIndex is unset the syncer
	// skips memory rows (fail-open) so the public catalog keeps working.
	var memoryIndex meilisearch.IndexManager
	if c.Meili.MemoryIndex != "" && c.Meili.MemoryIndex != c.Meili.Index {
		memoryIndex = client.Index(c.Meili.MemoryIndex)
		if _, err := client.GetIndex(c.Meili.MemoryIndex); err != nil {
			if _, cerr := client.CreateIndex(&meilisearch.IndexConfig{
				Uid:        c.Meili.MemoryIndex,
				PrimaryKey: "id",
			}); cerr != nil {
				logx.Must(fmt.Errorf("create meilisearch memory index: %w", cerr))
			}
		}

		if _, err := memoryIndex.UpdateSearchableAttributes(&[]string{"content"}); err != nil {
			logx.Must(fmt.Errorf("update memory searchable attributes: %w", err))
		}
		if _, err := memoryIndex.UpdateFilterableAttributes(&[]string{"user_id", "entity_type"}); err != nil {
			logx.Must(fmt.Errorf("update memory filterable attributes: %w", err))
		}
		if _, err := memoryIndex.UpdateSortableAttributes(&[]string{"created_at"}); err != nil {
			logx.Must(fmt.Errorf("update memory sortable attributes: %w", err))
		}

		// Hybrid search embedder. Meili auto-embeds docs (via documentTemplate)
		// and the query string, so the worker and the coach never compute
		// embeddings themselves. The Source is config-driven ("openAi" for an
		// OpenAI-compatible cloud endpoint, "ollama" for a local Ollama
		// instance) so the same code path serves both.
		embedderName := c.Meili.Embedder.Name
		if embedderName == "" {
			embedderName = "default"
		}
		docTpl := c.Meili.Embedder.DocumentTemplate
		if docTpl == "" {
			docTpl = "{{doc.content}}"
		}
		source := c.Meili.Embedder.Source
		if source == "" {
			source = "openAi"
		}
		embedders := map[string]meilisearch.Embedder{
			embedderName: {
				Source:           source,
				URL:              c.Meili.Embedder.URL,
				APIKey:           c.Meili.Embedder.APIKey,
				Model:            c.Meili.Embedder.Model,
				Dimensions:       c.Meili.Embedder.Dimensions,
				DocumentTemplate: docTpl,
			},
		}
		task, err := memoryIndex.UpdateEmbedders(embedders)
		if err != nil {
			logx.Must(fmt.Errorf("update memory embedders: %w", err))
		}
		if _, err := memoryIndex.WaitForTask(task.TaskUID, 5*time.Second); err != nil {
			logx.Must(fmt.Errorf("wait for memory embedders task: %w", err))
		}
	}

	return &ServiceContext{
		Config:      c,
		Pool:        pool,
		Meili:       client,
		Index:       index,
		MemoryIndex: memoryIndex,
	}
}

func (s *ServiceContext) Close() {
	if s.Pool != nil {
		s.Pool.Close()
	}
}
