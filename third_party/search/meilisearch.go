package search

import (
	"fmt"

	"github.com/meilisearch/meilisearch-go"
	"github.com/zeromicro/go-zero/core/logx"
)

type MeiliSearchConfig struct {
	Host      string
	MasterKey string
}

type MeiliSearchClient struct {
	client meilisearch.ServiceManager
}

func NewMeiliSearchConnection(config MeiliSearchConfig) (*MeiliSearchClient, error) {
	client := meilisearch.New(config.Host, meilisearch.WithAPIKey(config.MasterKey))

	// Test the connection by checking health
	_, err := client.Health()
	if err != nil {
		logx.Errorf("Failed to connect to Meilisearch: %v", err)
		return nil, fmt.Errorf("failed to connect to Meilisearch: %w", err)
	}

	logx.Info("Successfully connected to Meilisearch")
	return &MeiliSearchClient{client: client}, nil
}

func (m *MeiliSearchClient) GetClient() meilisearch.ServiceManager {
	return m.client
}

func (m *MeiliSearchClient) Close() error {
	// Meilisearch client doesn't require explicit cleanup
	// but we can add any cleanup logic here if needed
	return nil
}

// Helper methods for common operations
func (m *MeiliSearchClient) CreateIndex(indexName string, primaryKey string) error {
	_, err := m.client.CreateIndex(&meilisearch.IndexConfig{
		Uid:        indexName,
		PrimaryKey: primaryKey,
	})
	if err != nil {
		logx.Errorf("Failed to create index %s: %v", indexName, err)
		return fmt.Errorf("failed to create index %s: %w", indexName, err)
	}
	logx.Infof("Successfully created index: %s", indexName)
	return nil
}

func (m *MeiliSearchClient) AddDocuments(indexName string, documents interface{}) error {
	_, err := m.client.Index(indexName).AddDocuments(documents, nil)
	if err != nil {
		logx.Errorf("Failed to add documents to index %s: %v", indexName, err)
		return fmt.Errorf("failed to add documents to index %s: %w", indexName, err)
	}
	logx.Infof("Successfully added documents to index: %s", indexName)
	return nil
}

func (m *MeiliSearchClient) Search(indexName string, query string, limit int) (*meilisearch.SearchResponse, error) {
	searchRequest := &meilisearch.SearchRequest{
		Limit: int64(limit),
	}

	result, err := m.client.Index(indexName).Search(query, searchRequest)
	if err != nil {
		logx.Errorf("Failed to search in index %s: %v", indexName, err)
		return nil, fmt.Errorf("failed to search in index %s: %w", indexName, err)
	}

	return result, nil
}

// Index constants
const (
	ArticlesIndex      = "articles"
	HabitsIndex        = "habits"
	GoalsIndex         = "goals"
	ProfilesIndex      = "profiles"
	ConversationsIndex = "conversations"
)
