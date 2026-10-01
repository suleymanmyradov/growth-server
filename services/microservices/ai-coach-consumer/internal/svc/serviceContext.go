package svc

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/queue"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/ai/safety"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/pkg/events/outbox"
	"github.com/suleymanmyradov/growth-server/pkg/events/redisstream"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/pkg/redisutil"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach-consumer/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach-consumer/internal/consumer"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach-consumer/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach-consumer/internal/repository/db"
)

type ServiceContext struct {
	Config    config.Config
	Repo      *repository.Repository
	AI        ai.Client
	TxRunner  *postgres.PgxTxRunner
	EventsQ   queue.MessageQueue
	EventsPub *events.Publisher
	DLQPub    *events.DLQPublisher
	// EventRelay drains ai_coach_event_outbox into the events broker (P1).
	// Nil when no publisher backend (Kafka or Redis) is configured.
	EventRelay *outbox.Relay
	pool       *pgxpool.Pool
	consumerWg sync.WaitGroup
}

func NewServiceContext(c config.Config) *ServiceContext {
	pool := postgres.MustOpenPool(c.Postgres.Datasource, c.Postgres.MaxOpenConns, c.Postgres.MaxIdleConns, c.Postgres.ConnMaxLifetime)
	queries := db.New(pool)
	repo := repository.NewRepository(queries)
	txRunner := postgres.NewPgxTxRunner(pool)

	// AI client.
	var aiClient ai.Client
	var quotaRedisClient *redis.Client
	if c.AI.APIKey != "" {
		opts := []ai.Option{}
		if c.AI.Quota.RedisAddr != "" {
			// No startup ping: the client connects lazily. The quota store is
			// always wired when configured — if Redis is down, enforcement
			// fails closed at request time instead of silently disabling
			// quotas for the process lifetime.
			quotaRedisClient = redis.NewClient(redisutil.DefaultOpts(c.AI.Quota.RedisAddr, c.AI.Quota.RedisPassword, c.AI.Quota.RedisDB))
			opts = append(opts, ai.WithQuotaStore(ai.NewRedisQuotaStore(quotaRedisClient)))
		} else if c.AI.Quota.UserDailyTokenCap > 0 || c.AI.Quota.GlobalDailyCostCapUSD > 0 || c.AI.Quota.UserDailyVoiceSecondsCap > 0 {
			logx.Error("AI quota caps configured but AI.Quota.RedisAddr is empty; quota checks fail closed")
		}
		client, err := ai.New(c.AI, opts...)
		if err != nil {
			logx.Must(fmt.Errorf("failed to create AI client: %w", err))
		}
		aiClient = client
	}

	useKafka := len(c.Kafka.Brokers) > 0

	// Connect to Redis for Redis Streams fallback when Kafka is not configured.
	// Reuse the quota Redis client if it's the same address, otherwise create
	// a separate connection.
	var streamRedisClient *redis.Client
	if !useKafka && c.Redis.Addr != "" {
		if quotaRedisClient != nil && c.AI.Quota.RedisAddr == c.Redis.Addr {
			streamRedisClient = quotaRedisClient
		} else {
			client, err := redisutil.NewClient(c.Redis.Addr, c.Redis.Password, c.Redis.DB)
			if err != nil {
				logx.Errorf("redis unavailable; redis streams event transport disabled: %v", err)
			} else {
				streamRedisClient = client
			}
		}
	}

	// Events publisher (for publishing feedback events).
	var eventsPub *events.Publisher
	if useKafka && c.Kafka.EventsTopic != "" {
		eventsPub = events.NewPublisher(c.Kafka.Brokers, c.Kafka.EventsTopic)
	} else if streamRedisClient != nil && c.Kafka.EventsTopic != "" {
		eventsPub = events.NewRedisStreamPublisher(streamRedisClient, c.Kafka.EventsTopic)
	}

	// DLQ publisher.
	var dlqPub *events.DLQPublisher
	if useKafka && c.Kafka.DLQTopic != "" {
		dlqPub = events.NewDLQPublisher(c.Kafka.Brokers, c.Kafka.DLQTopic)
	} else if streamRedisClient != nil {
		dlqPub = events.NewRedisStreamDLQPublisher(streamRedisClient, events.DLQTopic)
	}

	// Safety classifier (if AI client is available).
	var classifier safety.Classifier
	if aiClient != nil {
		classifier = safety.NewLLMClassifier(aiClient)
	}

	// Build handler options.
	handlerOpts := &consumer.EventsHandlerOptions{
		TxRunner:    txRunner,
		DLQPub:      dlqPub,
		AITimeout:   c.Consumer.Timeout,
		Concurrency: c.Consumer.Concurrency,
		ServiceName: "ai-coach-consumer",
	}

	// Create consumer handler.
	handler := consumer.NewEventsHandler(repo, aiClient, classifier, handlerOpts)

	// Create consumer queue: Kafka or Redis Streams.
	var eventsQ queue.MessageQueue
	consumers := c.Kafka.Consumers
	if consumers == 0 {
		consumers = 8
	}
	if useKafka {
		kqConf := kq.KqConf{
			Brokers:     c.Kafka.Brokers,
			Group:       c.Kafka.ConsumerGroup + ".events",
			Topic:       c.Kafka.EventsTopic,
			ForceCommit: c.Kafka.ForceCommit,
			Processors:  c.Kafka.Processors,
			Consumers:   c.Kafka.Consumers,
		}
		if kqConf.Processors == 0 {
			kqConf.Processors = 8
		}
		if kqConf.Consumers == 0 {
			kqConf.Consumers = 8
		}
		eventsQ = kq.MustNewQueue(kqConf, kq.WithHandle(events.WithRetryConsume(dlqPub, events.RetryConsumeConfig{
			ServiceName: c.Kafka.ConsumerGroup + ".events",
		}, handler.Consume).Handle()))
	} else if streamRedisClient != nil {
		eventsQ = redisstream.MustNewQueue(streamRedisClient, redisstream.Config{
			Stream:    c.Kafka.EventsTopic,
			Group:     c.Kafka.ConsumerGroup + ".events",
			Consumers: consumers,
		}, events.WithRetryConsume(dlqPub, events.RetryConsumeConfig{
			ServiceName: c.Kafka.ConsumerGroup + ".events",
		}, handler.Consume))
	}

	// Transactional outbox relay: drains ai_coach_event_outbox — the digest
	// handler writes its check_in_feedback_generated rows inside the digest
	// transaction, this relay republishes them under stable event IDs (P1).
	var eventRelay *outbox.Relay
	if eventsPub != nil {
		eventRelay = outbox.NewRelay("ai-coach-consumer", consumerOutboxStore{q: queries}, eventsPub)
	}

	return &ServiceContext{
		Config:     c,
		Repo:       repo,
		AI:         aiClient,
		TxRunner:   txRunner,
		EventsQ:    eventsQ,
		EventsPub:  eventsPub,
		DLQPub:     dlqPub,
		EventRelay: eventRelay,
		pool:       pool,
	}
}

// consumerOutboxStore adapts the ai_coach_event_outbox queries to
// outbox.Store.
type consumerOutboxStore struct {
	q *db.Queries
}

func (s consumerOutboxStore) Claim(ctx context.Context) (outbox.Row, error) {
	row, err := s.q.ClaimEvent(ctx)
	if err != nil {
		return outbox.Row{}, err
	}
	return outbox.Row{
		EventID:    row.EventID,
		EventType:  row.EventType,
		Payload:    row.Payload,
		OccurredAt: row.OccurredAt.Time,
	}, nil
}

func (s consumerOutboxStore) Complete(ctx context.Context, eventID uuid.UUID) error {
	return s.q.CompleteEvent(ctx, eventID)
}

// WithTx returns a new Repository backed by the given transaction.
func (s *ServiceContext) WithTx(tx pgx.Tx) *repository.Repository {
	return repository.NewRepository(db.NewWithTx(tx))
}

// StartConsumers launches the event consumer queue and the outbox relay.
func (s *ServiceContext) StartConsumers() {
	if s.EventRelay != nil {
		s.consumerWg.Add(1)
		go func() {
			defer s.consumerWg.Done()
			s.EventRelay.Run(context.Background())
		}()
	}
	if s.EventsQ == nil {
		logx.Info("no event consumer configured; ai-coach-consumer idle")
		return
	}
	s.consumerWg.Add(1)
	go func() {
		defer s.consumerWg.Done()
		s.EventsQ.Start()
	}()
	logx.Info("started ai-coach event consumer")
}

func (s *ServiceContext) Close() {
	if s.EventsQ != nil {
		s.EventsQ.Stop()
	}
	s.consumerWg.Wait()
	if s.EventsPub != nil {
		_ = s.EventsPub.Close()
	}
	if s.DLQPub != nil {
		_ = s.DLQPub.Close()
	}
	if s.pool != nil {
		s.pool.Close()
	}
}
