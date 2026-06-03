package svc

import (
	"fmt"

	"github.com/meilisearch/meilisearch-go"
	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/internal/config"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceContext struct {
	Config config.Config
	Meili  meilisearch.ServiceManager
	Index  meilisearch.IndexManager
}

func NewServiceContext(c config.Config) *ServiceContext {
	client := meilisearch.New(c.Meili.Host, meilisearch.WithAPIKey(c.Meili.APIKey))

	indexName := c.Meili.Index
	if indexName == "" {
		indexName = "growth_search"
	}

	_, err := client.GetIndex(indexName)
	if err != nil {
		_, err = client.CreateIndex(&meilisearch.IndexConfig{
			Uid:        indexName,
			PrimaryKey: "id",
		})
		if err != nil {
			logx.Must(fmt.Errorf("create meilisearch index: %w", err))
		}
	}

	return &ServiceContext{
		Config: c,
		Meili:  client,
		Index:  client.Index(indexName),
	}
}
