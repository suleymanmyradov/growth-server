package svc

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/queue"

	"github.com/suleymanmyradov/growth-server/pkg/email"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/pkg/events/outbox"
	"github.com/suleymanmyradov/growth-server/pkg/events/redisstream"
	expo "github.com/suleymanmyradov/growth-server/pkg/notifications/expo"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/pkg/redisutil"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/consumer"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/delivery"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/scheduler"
)

type ServiceContext struct {
	Config       config.Config
	Repo         *repository.Repository
	EventsPub    *events.Publisher
	ReminderPub  *events.Publisher
	Scheduler    *scheduler.Scheduler
	TxRunner     *postgres.PgxTxRunner
	EventsQ      queue.MessageQueue
	ReminderDueQ queue.MessageQueue
	// PushSender delivers push notifications via Expo. Nil-safe: when Expo is
	// disabled in config, Send is a no-op. See docs/push-notifications-design.md.
	PushSender *delivery.Sender
	// ReceiptWorker checks Expo push receipts asynchronously and disables
	// stale tokens. Nil-safe: when Expo is disabled, Run is a no-op.
	ReceiptWorker  *delivery.ReceiptWorker
	DeliveryWorker *delivery.Worker
	// EmailUnsubscribeSecret verifies the RFC 8058 tokens in List-Unsubscribe
	// URLs (UnsubscribeEmail RPC). Resolved from Email.UnsubscribeSecret with
	// ServiceAuth.Secret as fallback; empty when email delivery is disabled.
	EmailUnsubscribeSecret string
	// EventRelay drains notification_event_outbox into the events broker (P1).
	// Nil when no publisher backend is configured.
	EventRelay  *outbox.Relay
	pool        *pgxpool.Pool
	schedCancel context.CancelFunc
}

func mustOpenDB(datasource string, maxOpen, maxIdle int, maxLifetime time.Duration) *pgxpool.Pool {
	config, err := pgxpool.ParseConfig(datasource)
	if err != nil {
		panic(fmt.Errorf("parse pgx config: %w", err))
	}
	config.MaxConns = int32(maxOpen)
	config.MinConns = int32(maxIdle)
	config.MaxConnLifetime = maxLifetime

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		panic(fmt.Errorf("pgx pool: %w", err))
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		panic(fmt.Errorf("pgx ping: %w", err))
	}
	return pool
}

func NewServiceContext(c config.Config) *ServiceContext {
	pool := mustOpenDB(c.Postgres.Datasource, c.Postgres.MaxOpenConns, c.Postgres.MaxIdleConns, c.Postgres.ConnMaxLifetime)
	queries := db.New(pool)
	repo := repository.NewRepository(queries)
	txRunner := postgres.NewPgxTxRunner(pool)

	useKafka := len(c.Kafka.Brokers) > 0

	// Connect to Redis for Redis Streams fallback when Kafka is not configured.
	var redisClient *redis.Client
	if !useKafka && c.Redis.Addr != "" {
		client, err := redisutil.NewClient(c.Redis.Addr, c.Redis.Password, c.Redis.DB)
		if err != nil {
			logx.Errorf("redis unavailable; redis streams event transport disabled: %v", err)
		} else {
			redisClient = client
		}
	}

	// Publishers: use Kafka when brokers are configured, Redis Streams as
	// fallback, or nil (no-op) when neither is available.
	var reminderPub *events.Publisher
	var dlqPub *events.DLQPublisher
	var eventsPub *events.Publisher

	if useKafka {
		reminderPub = events.NewPublisher(c.Kafka.Brokers, c.Kafka.ReminderDueTopic)
		dlqPub = events.NewDLQPublisher(c.Kafka.Brokers, events.DLQTopic)
		if c.Kafka.EventsTopic != "" {
			eventsPub = events.NewPublisher(c.Kafka.Brokers, c.Kafka.EventsTopic)
		}
	} else if redisClient != nil {
		if c.Kafka.ReminderDueTopic != "" {
			reminderPub = events.NewRedisStreamPublisher(redisClient, c.Kafka.ReminderDueTopic)
		}
		dlqPub = events.NewRedisStreamDLQPublisher(redisClient, events.DLQTopic)
		if c.Kafka.EventsTopic != "" {
			eventsPub = events.NewRedisStreamPublisher(redisClient, c.Kafka.EventsTopic)
		}
	}

	sched := scheduler.NewScheduler(repo.Reminders, reminderPub, realClock{})

	// Expo push client + delivery sender. When Expo is disabled in config,
	// the sender is still constructed (with a nil Expo client) so callers can
	// invoke Send without nil-checks; it is a no-op in that case.
	var expoClient *expo.Client
	if c.Expo.Enabled {
		expoClient = expo.NewClient(nil, c.Expo.AccessToken)
	}
	pushSender := delivery.NewSender(repo.Devices, repo.PushTickets, expoClient, c.Expo.Enabled)
	receiptWorker := delivery.NewReceiptWorker(repo.Devices, repo.PushTickets, expoClient)
	var emailSender email.Sender
	var unsubscribeSecret string
	if c.Email.Enabled {
		if c.Email.FrontendBaseURL == "" {
			panic("email frontend base URL is required when email delivery is enabled")
		}
		if c.Email.APIBaseURL == "" {
			panic("email API base URL is required when email delivery is enabled")
		}
		// Without a secret the unsubscribe tokens are forgeable — anyone could
		// disable email for arbitrary users — so fail fast rather than send
		// emails with unverifiable links.
		unsubscribeSecret = c.Email.UnsubscribeSecret
		if unsubscribeSecret == "" {
			unsubscribeSecret = c.ServiceAuth.Secret
		}
		if unsubscribeSecret == "" {
			panic("email unsubscribe secret is required when email delivery is enabled (set Email.UnsubscribeSecret or ServiceAuth.Secret)")
		}
		var err error
		emailSender, err = email.New(email.Config{
			Provider:    c.Email.Provider,
			APIKey:      c.Email.APIKey,
			FromAddress: c.Email.FromAddress,
		})
		if err != nil {
			panic(fmt.Errorf("create email sender: %w", err))
		}
	}
	deliveryWorker := delivery.NewWorker(repo, pushSender, emailSender, c.Email.FrontendBaseURL, c.Email.APIBaseURL, unsubscribeSecret)

	eventsHandler := consumer.NewEventsHandler(repo, reminderPub, nil, txRunner, dlqPub)
	reminderDueHandler := consumer.NewReminderDueHandler(repo, nil, txRunner, dlqPub, pushSender, eventsPub)

	// Consumer queues: Kafka when brokers are configured, Redis Streams as
	// fallback, or nil (no consumers) when neither is available.
	var eventsQ queue.MessageQueue
	var reminderDueQ queue.MessageQueue

	group := c.Kafka.ConsumerGroup
	if group == "" {
		group = "notifications"
	}

	if useKafka {
		eventsQ = kq.MustNewQueue(
			kq.KqConf{
				Brokers:    c.Kafka.Brokers,
				Group:      group + ".events",
				Topic:      c.Kafka.EventsTopic,
				Offset:     "first",
				Consumers:  8,
				Processors: 8,
			},
			kq.WithHandle(events.WithRetryConsume(dlqPub, events.RetryConsumeConfig{
				ServiceName: group + ".events",
			}, eventsHandler.Consume).Handle()),
		)
		reminderDueQ = kq.MustNewQueue(
			kq.KqConf{
				Brokers:    c.Kafka.Brokers,
				Group:      group + ".reminders",
				Topic:      c.Kafka.ReminderDueTopic,
				Offset:     "first",
				Consumers:  8,
				Processors: 8,
			},
			kq.WithHandle(events.WithRetryConsume(dlqPub, events.RetryConsumeConfig{
				ServiceName: group + ".reminders",
			}, reminderDueHandler.Consume).Handle()),
		)
	} else if redisClient != nil {
		if c.Kafka.EventsTopic != "" {
			eventsQ = redisstream.MustNewQueue(redisClient, redisstream.Config{
				Stream:    c.Kafka.EventsTopic,
				Group:     group + ".events",
				Consumers: 8,
			}, events.WithRetryConsume(dlqPub, events.RetryConsumeConfig{
				ServiceName: group + ".events",
			}, eventsHandler.Consume))
		}
		if c.Kafka.ReminderDueTopic != "" {
			reminderDueQ = redisstream.MustNewQueue(redisClient, redisstream.Config{
				Stream:    c.Kafka.ReminderDueTopic,
				Group:     group + ".reminders",
				Consumers: 8,
			}, events.WithRetryConsume(dlqPub, events.RetryConsumeConfig{
				ServiceName: group + ".reminders",
			}, reminderDueHandler.Consume))
		}
	}

	// Transactional outbox relay: coach_digest and other domain events are
	// written to notification_event_outbox inside the dispatch transaction;
	// the relay republishes them under stable event IDs (P1).
	var eventRelay *outbox.Relay
	if eventsPub != nil {
		eventRelay = outbox.NewRelay("notifications", notificationOutboxStore{q: queries}, eventsPub)
	}

	return &ServiceContext{
		Config:                 c,
		Repo:                   repo,
		EventsPub:              eventsPub,
		ReminderPub:            reminderPub,
		Scheduler:              sched,
		TxRunner:               txRunner,
		EventsQ:                eventsQ,
		ReminderDueQ:           reminderDueQ,
		PushSender:             pushSender,
		ReceiptWorker:          receiptWorker,
		DeliveryWorker:         deliveryWorker,
		EmailUnsubscribeSecret: unsubscribeSecret,
		EventRelay:             eventRelay,
		pool:                   pool,
	}
}

// notificationOutboxStore adapts the sqlc notification_event_outbox queries to
// outbox.Store.
type notificationOutboxStore struct {
	q *db.Queries
}

func (s notificationOutboxStore) Claim(ctx context.Context) (outbox.Row, error) {
	row, err := s.q.ClaimNotificationEvent(ctx)
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

func (s notificationOutboxStore) Complete(ctx context.Context, eventID uuid.UUID) error {
	return s.q.CompleteNotificationEvent(ctx, eventID)
}

// WithTx returns a new Repository backed by the given transaction.
func (s *ServiceContext) WithTx(tx pgx.Tx) *repository.Repository {
	return repository.NewRepository(db.New(tx))
}

// StartConsumers launches the scheduler goroutine and both event queues.
func (s *ServiceContext) StartConsumers() context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	s.schedCancel = cancel

	go s.Scheduler.Run(ctx)
	if s.EventsQ != nil {
		go s.EventsQ.Start()
	}
	if s.ReminderDueQ != nil {
		go s.ReminderDueQ.Start()
	}
	if s.ReceiptWorker != nil {
		go s.ReceiptWorker.Run(ctx)
	}
	if s.DeliveryWorker != nil {
		go s.DeliveryWorker.Run(ctx)
	}
	if s.EventRelay != nil {
		go s.EventRelay.Run(ctx)
	}

	logx.Info("started scheduler, event consumers, and notification delivery workers")
	return cancel
}

func (s *ServiceContext) Close() {
	if s.schedCancel != nil {
		s.schedCancel()
	}
	if s.EventsQ != nil {
		s.EventsQ.Stop()
	}
	if s.ReminderDueQ != nil {
		s.ReminderDueQ.Stop()
	}
	if s.EventsPub != nil {
		_ = s.EventsPub.Close()
	}
	if s.ReminderPub != nil {
		_ = s.ReminderPub.Close()
	}
	if s.pool != nil {
		s.pool.Close()
	}
}

// realClock implements consumer.Clock and scheduler.Clock.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }
