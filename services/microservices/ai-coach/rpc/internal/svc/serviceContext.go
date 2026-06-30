package svc

import (
	"fmt"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/ai/safety"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/pkg/redisutil"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceContext struct {
	Config     config.Config
	AIClient   ai.Client
	Classifier safety.Classifier
	Queries    *db.Queries
	TxRunner   *postgres.PgxTxRunner
}

func NewServiceContext(c config.Config) *ServiceContext {
	var aiClient ai.Client
	if c.AI.APIKey != "" {
		opts := []ai.Option{}
		if c.AI.Quota.RedisAddr != "" {
			redisClient, err := redisutil.NewClient(c.AI.Quota.RedisAddr, c.AI.Quota.RedisPassword, c.AI.Quota.RedisDB)
			if err == nil {
				opts = append(opts, ai.WithQuotaStore(ai.NewRedisQuotaStore(redisClient)))
			} else {
				logx.Errorf("redis unavailable; AI quotas disabled: %v", err)
			}
		}
		client, err := ai.New(c.AI, opts...)
		if err != nil {
			logx.Must(fmt.Errorf("failed to create AI client: %w", err))
		}
		aiClient = client
	}

	// Safety classifier (if AI client is available). Used to pre-screen user
	// input on the coaching path before it reaches the model.
	var classifier safety.Classifier
	if aiClient != nil {
		classifier = safety.NewLLMClassifier(aiClient)
	}

	// Postgres connection for conversation persistence. Optional — if not
	// configured, conversation RPCs will return Unavailable errors but AI
	// coaching/check-in/weekly-review features still work.
	var queries *db.Queries
	var txRunner *postgres.PgxTxRunner
	if c.Postgres.Datasource != "" {
		pool := postgres.MustOpenPool(c.Postgres.Datasource, c.Postgres.MaxOpenConns, c.Postgres.MaxIdleConns, c.Postgres.ConnMaxLifetime)
		queries = db.New(pool)
		txRunner = postgres.NewPgxTxRunner(pool)
	}

	return &ServiceContext{
		Config:     c,
		AIClient:   aiClient,
		Classifier: classifier,
		Queries:    queries,
		TxRunner:   txRunner,
	}
}
