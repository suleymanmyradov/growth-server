package svc

import (
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/ai/safety"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/pkg/redisutil"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach-consumer/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach-consumer/internal/consumer"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach-consumer/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach-consumer/internal/repository/db"
	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/queue"
)

type ServiceContext struct {
	Config     config.Config
	Repo       *repository.Repository
	AI         ai.Client
	TxRunner   *postgres.PgxTxRunner
	EventsQ    queue.MessageQueue
	EventsPub  *events.Publisher
	DLQPub     *events.DLQPublisher
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
			logx.Errorf("failed to create AI client: %v", err)
		} else {
			aiClient = client
		}
	}

	// Events publisher (for publishing feedback events).
	var eventsPub *events.Publisher
	if len(c.Kafka.Brokers) > 0 && c.Kafka.EventsTopic != "" {
		eventsPub = events.NewPublisher(c.Kafka.Brokers, c.Kafka.EventsTopic)
	}

	// DLQ publisher.
	var dlqPub *events.DLQPublisher
	if len(c.Kafka.Brokers) > 0 && c.Kafka.DLQTopic != "" {
		dlqPub = events.NewDLQPublisher(c.Kafka.Brokers, c.Kafka.DLQTopic)
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
	handler := consumer.NewEventsHandler(repo, aiClient, eventsPub, classifier, handlerOpts)

	// Create kq queue.
	// ForceCommit=false so that handler errors cause the message to be
	// redelivered instead of silently committed.
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
	eventsQ := kq.MustNewQueue(kqConf, kq.WithHandle(handler.Consume))

	return &ServiceContext{
		Config:    c,
		Repo:      repo,
		AI:        aiClient,
		TxRunner:  txRunner,
		EventsQ:   eventsQ,
		EventsPub: eventsPub,
		DLQPub:    dlqPub,
		pool:      pool,
	}
}

// WithTx returns a new Repository backed by the given transaction.
func (s *ServiceContext) WithTx(tx pgx.Tx) *repository.Repository {
	return repository.NewRepository(db.NewWithTx(tx))
}

// StartConsumers launches the kq queue.
func (s *ServiceContext) StartConsumers() {
	s.consumerWg.Add(1)
	go func() {
		defer s.consumerWg.Done()
		s.EventsQ.Start()
	}()
	logx.Info("started ai-coach kafka consumer")
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
