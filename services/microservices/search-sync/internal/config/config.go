package config

import (
	"time"

	"github.com/zeromicro/go-zero/core/service"
)

type Config struct {
	service.ServiceConf
	Postgres struct {
		Datasource      string `json:",optional" secret:"true"`
		MaxOpenConns    int
		MaxIdleConns    int
		ConnMaxLifetime time.Duration
	}
	Meili struct {
		Host   string
		APIKey string `json:",optional" secret:"true"`
		Index  string
		// MemoryIndex is the SEPARATE, private index holding per-user
		// free-text memories for the AI coach (check-in notes, conversation
		// messages, weekly-review summaries). It must never be the same as
		// the public catalog Index.
		MemoryIndex string `json:",optional"`
		// Embedder configures the Meili `source: openAi` embedder used for
		// hybrid search on the memory index. Meili auto-embeds documents at
		// index time and the query string at search time, so no Go-side
		// embedding code is needed. The URL may point at any OpenAI-compatible
		// /embeddings endpoint (OpenAI, NVIDIA integrate, etc.).
		Embedder EmbedderConfig `json:",optional"`
	}
	Sync struct {
		PollInterval time.Duration
		BatchSize    int
		LockTimeout  time.Duration
		MaxAttempts  int
		WorkerID     string
	}
	Backfill bool
}

// EmbedderConfig configures the embedder on the memory index. Meili
// auto-embeds documents (via DocumentTemplate) and the query string, so the
// worker and the coach never compute embeddings themselves.
type EmbedderConfig struct {
	Name             string `json:",optional"` // embedder key used in hybrid search; defaults to "default"
	Source           string `json:",optional"` // Meili embedder source: "openAi" or "ollama"; defaults to "openAi"
	URL              string `json:",optional"` // embeddings base URL (OpenAI-compatible /v1, or Ollama http://host:11434)
	APIKey           string `json:",optional" secret:"true"`
	Model            string `json:",optional"` // e.g. "text-embedding-3-small" or "embeddinggemma:300m"
	Dimensions       int    `json:",optional"` // must match the model's output dim
	DocumentTemplate string `json:",optional"` // Meili template rendered for embedding; defaults to {{doc.content}}
}
