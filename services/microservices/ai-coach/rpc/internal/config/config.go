package config

import (
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	AI ai.Config
	Postgres struct {
		Datasource      string `json:",optional" secret:"true"`
		MaxOpenConns    int
		MaxIdleConns    int
		ConnMaxLifetime time.Duration
	}
	// Meili is the read-only connection to the private user_memory index used
	// for long-term coaching memory retrieval. Optional; if Host is empty,
	// retrieval is disabled and coaching behaves exactly as before.
	Meili struct {
		Host        string `json:",optional"`
		APIKey      string `json:",optional" secret:"true"`
		MemoryIndex string `json:",optional"`
	}
	// CoachMemory controls long-term memory retrieval (Workstream 2). It is a
	// feature flag so retrieval can be A/B'd and disabled without a deploy.
	CoachMemory CoachMemoryConfig `json:",optional"`
}

// CoachMemoryConfig controls the retrieval enhancement. Retrieval is always
// fail-open: on any Meili error/timeout the coach proceeds without retrieval.
type CoachMemoryConfig struct {
	Enabled         bool          `json:",optional"` // feature flag for A/B
	EmbedderName    string        `json:",optional"` // hybrid embedder key; default "default"
	SemanticRatio   float64       `json:",optional"` // 0..1; default 0.5
	Limit           int           `json:",optional"` // max snippets; default 5
	ScoreFloor      float64       `json:",optional"` // min _rankingScore; default 0.2
	Timeout         time.Duration `json:",optional"` // per-query deadline; default 300ms
	MaxSnippetChars int           `json:",optional"` // per-snippet truncate; default 240
}
