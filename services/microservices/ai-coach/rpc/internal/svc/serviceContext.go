package svc

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/meilisearch/meilisearch-go"
	"github.com/redis/go-redis/v9"
	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/ai/safety"
	"github.com/suleymanmyradov/growth-server/pkg/events/userdeletion"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/pkg/redisutil"
	"github.com/suleymanmyradov/growth-server/pkg/speech"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/facts"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/memory"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/queue"
)

type ServiceContext struct {
	Config          config.Config
	AIClient        ai.Client
	Classifier      safety.Classifier
	Queries         *db.Queries
	TxRunner        *postgres.PgxTxRunner
	MemoryRetriever *memory.Retriever
	// FactStore is the curated tier of long-term memory (user_facts). Nil when
	// Postgres is unconfigured; callers treat nil as "curated memory disabled".
	FactStore *facts.Store
	// FactExtractor turns a completed exchange into fact candidates. Nil when
	// the AI client is unconfigured.
	FactExtractor *facts.Extractor
	// Speech clients (optional). Nil when Speech config is empty — the
	// Transcribe/Synthesize RPCs then return Unavailable.
	STT speech.STTClient
	TTS speech.TTSClient
	// QuotaStore backs the per-user voice (STT seconds) quota enforced by
	// Transcribe. Nil when AI.Quota.RedisAddr is unset or AI is disabled.
	QuotaStore ai.QuotaStore
	// DeletionQ consumes user_deleted events and wipes curated memory facts
	// (user_facts). Nil when UserDeletion.Topic is empty.
	DeletionQ     queue.MessageQueue
	closeDeletion func()
}

func NewServiceContext(c config.Config) *ServiceContext {
	var aiClient ai.Client
	var quotaStore ai.QuotaStore
	if c.AI.APIKey != "" {
		opts := []ai.Option{}
		if c.AI.Quota.RedisAddr != "" {
			// No startup ping: the client connects lazily. The quota store is
			// always wired when configured — if Redis is down, enforcement
			// fails closed at request time instead of silently disabling
			// quotas for the process lifetime.
			quotaStore = ai.NewRedisQuotaStore(redis.NewClient(redisutil.DefaultOpts(c.AI.Quota.RedisAddr, c.AI.Quota.RedisPassword, c.AI.Quota.RedisDB)))
			opts = append(opts, ai.WithQuotaStore(quotaStore))
		} else if c.AI.Quota.UserDailyTokenCap > 0 || c.AI.Quota.GlobalDailyCostCapUSD > 0 || c.AI.Quota.UserDailyVoiceSecondsCap > 0 {
			logx.Errorf("AI quota caps configured but AI.Quota.RedisAddr is empty; quota checks fail closed")
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

	// Long-term memory retrieval (Workstream 2). Read-only client over the
	// private user_memory index. Optional: if Meili.Host or MemoryIndex is
	// empty, retrieval stays disabled and coaching behaves as before.
	var memoryRetriever *memory.Retriever
	if c.Meili.Host != "" && c.Meili.MemoryIndex != "" {
		meiliClient := meilisearch.New(c.Meili.Host, meilisearch.WithAPIKey(c.Meili.APIKey))
		memoryRetriever = memory.NewRetriever(meiliClient.Index(c.Meili.MemoryIndex), memory.Config{
			EmbedderName:  c.CoachMemory.EmbedderName,
			SemanticRatio: c.CoachMemory.SemanticRatio,
			Limit:         c.CoachMemory.Limit,
			ScoreFloor:    c.CoachMemory.ScoreFloor,
		})
	}

	// Curated long-term memory. The store needs Postgres; the extractor needs
	// the AI client. Either can be absent independently: without the store
	// nothing is remembered, without the extractor nothing new is learned but
	// existing facts still reach the prompt.
	var factStore *facts.Store
	if queries != nil {
		factStore = facts.NewStore(queries, facts.Config{
			MinConfidence: c.CoachMemory.FactMinConfidence,
			MaxFacts:      c.CoachMemory.FactMaxCount,
		})
	}
	factExtractor := facts.NewExtractor(aiClient)

	// Speech (STT/TTS) for dictate + live voice chat. Optional: if Speech.APIKey
	// is empty, both clients are nil and the Transcribe/Synthesize RPCs return
	// Unavailable. Reuses the OpenRouter key by default (same provider as chat).
	speechClients, err := speech.New(c.Speech)
	if err != nil {
		logx.Must(fmt.Errorf("failed to create speech clients: %w", err))
	}

	// user_deleted consumer: conversations, messages (cascading), and curated
	// facts live in this service, so the deletion cleanup belongs here (the
	// ai-coach-consumer only wipes its own ai_feedback/read-model tables).
	// Disabled when UserDeletion.Topic is empty.
	deletionQ, closeDeletion, err := userdeletion.NewQueue(c.UserDeletion, userdeletion.Handler{
		Delete: func(ctx context.Context, userID uuid.UUID) error {
			if queries == nil {
				return errors.New("user deletion cleanup unavailable: postgres not configured")
			}
			if err := queries.ForgetAllUserFacts(ctx, userID); err != nil {
				return fmt.Errorf("forget user facts: %w", err)
			}
			if err := queries.DeleteConversationsByUser(ctx, userID); err != nil {
				return fmt.Errorf("delete conversations: %w", err)
			}
			return nil
		},
	})
	if err != nil {
		logx.Must(fmt.Errorf("failed to create user deletion queue: %w", err))
	}

	return &ServiceContext{
		Config:          c,
		AIClient:        aiClient,
		Classifier:      classifier,
		Queries:         queries,
		TxRunner:        txRunner,
		MemoryRetriever: memoryRetriever,
		FactStore:       factStore,
		FactExtractor:   factExtractor,
		STT:             speechClients.STT,
		TTS:             speechClients.TTS,
		QuotaStore:      quotaStore,
		DeletionQ:       deletionQ,
		closeDeletion:   closeDeletion,
	}
}

// StartDeletionConsumer starts the user_deleted consumer in the background.
// No-op when the queue is disabled.
func (s *ServiceContext) StartDeletionConsumer() {
	if s.DeletionQ != nil {
		go s.DeletionQ.Start()
	}
}

// CloseDeletionConsumer stops the consumer and releases its transport
// resources. Safe to call when disabled.
func (s *ServiceContext) CloseDeletionConsumer() {
	if s.DeletionQ != nil {
		s.DeletionQ.Stop()
	}
	if s.closeDeletion != nil {
		s.closeDeletion()
	}
}
