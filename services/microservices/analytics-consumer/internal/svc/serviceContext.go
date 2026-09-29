package svc

import (
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/queue"

	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/pkg/events/redisstream"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/pkg/redisutil"
	"github.com/suleymanmyradov/growth-server/services/microservices/analytics-consumer/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/analytics-consumer/internal/consumer"
	"github.com/suleymanmyradov/growth-server/services/microservices/analytics-consumer/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/analytics-consumer/internal/repository/db"
)

type ServiceContext struct {
	Config     config.Config
	Repo       *repository.Repository
	EventsQ    queue.MessageQueue
	pool       *pgxpool.Pool
	dlqPub     *events.DLQPublisher
	consumerWg sync.WaitGroup
}

func NewServiceContext(c config.Config) *ServiceContext {
	pool := postgres.MustOpenPool(c.Postgres.Datasource, c.Postgres.MaxOpenConns, c.Postgres.MaxIdleConns, c.Postgres.ConnMaxLifetime)
	queries := db.New(pool)
	repo := repository.NewRepository(queries)

	handler := consumer.NewEventsHandler(repo)

	// Create kq queue.
	kqConf := kq.KqConf{
		Brokers:    c.Kafka.Brokers,
		Group:      c.Kafka.ConsumerGroup + ".events",
		Topic:      c.Kafka.EventsTopic,
		Offset:     "first",
		Processors: c.Kafka.Processors,
		Consumers:  c.Kafka.Consumers,
	}
	if kqConf.Processors == 0 {
		kqConf.Processors = 4
	}
	if kqConf.Consumers == 0 {
		kqConf.Consumers = 4
	}
	if kqConf.Group == ".events" {
		kqConf.Group = "analytics-consumer.events"
	}

	var eventsQ queue.MessageQueue
	var dlqPub *events.DLQPublisher
	serviceName := kqConf.Group
	if len(c.Kafka.Brokers) > 0 && c.Kafka.EventsTopic != "" {
		// kq auto-commits fetched offsets regardless of handler errors, so
		// the retry+DLQ wrapper is what keeps transient failures alive.
		dlqPub = events.NewDLQPublisher(c.Kafka.Brokers, events.DLQTopic)
		eventsQ = kq.MustNewQueue(kqConf, kq.WithHandle(events.WithRetryConsume(dlqPub, events.RetryConsumeConfig{
			ServiceName: serviceName,
		}, handler.Consume).Handle()))
	} else if c.Redis.Addr != "" && c.Kafka.EventsTopic != "" {
		redisClient, err := redisutil.NewClient(c.Redis.Addr, c.Redis.Password, c.Redis.DB)
		if err != nil {
			logx.Errorf("redis unavailable; analytics consumer disabled: %v", err)
		} else {
			dlqPub = events.NewRedisStreamDLQPublisher(redisClient, events.DLQTopic)
			eventsQ = redisstream.MustNewQueue(redisClient, redisstream.Config{
				Stream:    c.Kafka.EventsTopic,
				Group:     serviceName,
				Consumers: 4,
			}, events.WithRetryConsume(dlqPub, events.RetryConsumeConfig{
				ServiceName: serviceName,
			}, handler.Consume))
		}
	}

	return &ServiceContext{
		Config:  c,
		Repo:    repo,
		EventsQ: eventsQ,
		pool:    pool,
		dlqPub:  dlqPub,
	}
}

func (s *ServiceContext) StartConsumers() {
	if s.EventsQ != nil {
		s.consumerWg.Add(1)
		go func() {
			defer s.consumerWg.Done()
			s.EventsQ.Start()
		}()
		logx.Info("started analytics kafka consumer")
	}
}

func (s *ServiceContext) Close() {
	if s.EventsQ != nil {
		s.EventsQ.Stop()
	}
	s.consumerWg.Wait()
	if s.dlqPub != nil {
		_ = s.dlqPub.Close()
	}
	if s.pool != nil {
		s.pool.Close()
	}
}
