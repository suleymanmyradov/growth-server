// Package redisstream provides a Redis Streams-based message queue that
// implements the go-zero queue.MessageQueue interface. It is a drop-in
// replacement for kq.MustNewQueue when Kafka is not available.
//
// The queue uses Redis consumer groups for at-least-once delivery. Each
// consumer in a group receives a subset of messages; messages are
// acknowledged after the handler succeeds. Failed messages remain pending
// and are reclaimed by other consumers after the claim timeout.
package redisstream

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/queue"
	"github.com/zeromicro/go-zero/core/threading"
)

// ConsumeHandler matches the kq.ConsumeHandler interface so the same
// handler implementations work with both Kafka and Redis Streams.
type ConsumeHandler interface {
	Consume(ctx context.Context, key, value string) error
}

// Config configures a Redis Streams queue.
type Config struct {
	// Stream is the Redis Stream name (equivalent to Kafka topic).
	Stream string
	// Group is the consumer group name (equivalent to Kafka consumer group).
	Group string
	// Consumers is the number of concurrent consumer goroutines.
	Consumers int
	// Block is how long XREADGROUP blocks waiting for new messages.
	// Defaults to 5s if zero.
	Block time.Duration
	// ClaimMinIdleTime is how long a message must be pending before another
	// consumer can claim it. Defaults to 30s if zero.
	ClaimMinIdleTime time.Duration
	// MaxPollCount is the max number of messages to read per XREADGROUP call.
	// Defaults to 10 if zero.
	MaxPollCount int64
}

// Queue is a Redis Streams-based message queue implementing queue.MessageQueue.
type Queue struct {
	client  *redis.Client
	cfg     Config
	handler ConsumeHandler

	cancel context.CancelFunc
	wg     sync.WaitGroup
	once   sync.Once
}

// NewQueue creates a new Redis Streams queue. The client must already be
// connected. The queue creates the consumer group on Start if it doesn't
// exist (using MKSTREAM).
func NewQueue(client *redis.Client, cfg Config, handler ConsumeHandler) queue.MessageQueue {
	if cfg.Consumers <= 0 {
		cfg.Consumers = 8
	}
	if cfg.Block <= 0 {
		cfg.Block = 5 * time.Second
	}
	if cfg.ClaimMinIdleTime <= 0 {
		cfg.ClaimMinIdleTime = 30 * time.Second
	}
	if cfg.MaxPollCount <= 0 {
		cfg.MaxPollCount = 10
	}
	return &Queue{
		client:  client,
		cfg:     cfg,
		handler: handler,
	}
}

// MustNewQueue is like NewQueue but panics on error. Since NewQueue doesn't
// return an error (group creation is deferred to Start), this is equivalent
// to NewQueue but matches the kq.MustNewQueue signature for easy swapping.
func MustNewQueue(client *redis.Client, cfg Config, handler ConsumeHandler) queue.MessageQueue {
	return NewQueue(client, cfg, handler)
}

// Start launches the consumer goroutines. It creates the consumer group if
// it doesn't exist, then starts reading messages via XREADGROUP.
func (q *Queue) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	q.cancel = cancel

	// Create the consumer group if it doesn't exist. XGROUP CREATE with
	// MKSTREAM creates the stream if needed. ID "0" means read all existing
	// messages from the start; "$" means only new messages.
	// We use "0" so a fresh consumer group backfills from the beginning,
	// matching kq's Offset: "first" behavior.
	err := q.client.XGroupCreateMkStream(ctx, q.cfg.Stream, q.cfg.Group, "0").Err()
	if err != nil && !isBusyGroupError(err) {
		logx.Errorf("redisstream: create group %s on stream %s: %v", q.cfg.Group, q.cfg.Stream, err)
		// Don't return — the group might already exist from a previous run.
	}

	for i := 0; i < q.cfg.Consumers; i++ {
		q.wg.Add(1)
		threading.GoSafe(func() {
			defer q.wg.Done()
			q.consumeLoop(ctx, fmt.Sprintf("consumer-%d", i))
		})
	}
}

// Stop signals all consumer goroutines to stop and waits for them to finish.
func (q *Queue) Stop() {
	q.once.Do(func() {
		if q.cancel != nil {
			q.cancel()
		}
	})
	q.wg.Wait()
}

// consumeLoop reads messages from the stream via XREADGROUP, calls the
// handler, and acknowledges on success. On handler error, the message
// remains pending and will be reclaimed by another consumer after
// ClaimMinIdleTime.
func (q *Queue) consumeLoop(ctx context.Context, consumer string) {
	// Track whether we have pending messages to claim from crashed consumers.
	claimTicker := time.NewTicker(q.cfg.ClaimMinIdleTime)
	defer claimTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-claimTicker.C:
			q.claimPending(ctx, consumer)
			continue
		default:
		}

		// Read new messages with a blocking call.
		msgs, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    q.cfg.Group,
			Consumer: consumer,
			Streams:  []string{q.cfg.Stream, ">"},
			Count:    q.cfg.MaxPollCount,
			Block:    q.cfg.Block,
		}).Result()

		if err != nil {
			if err == redis.Nil || ctx.Err() != nil {
				// No messages available or context cancelled — continue.
				continue
			}
			logx.Errorf("redisstream: xreadgroup on stream %s: %v", q.cfg.Stream, err)
			time.Sleep(time.Second) // Back off on error.
			continue
		}

		for _, stream := range msgs {
			for _, msg := range stream.Messages {
				q.processMessage(ctx, consumer, msg)
			}
		}
	}
}

// claimPending uses XAUTOCLAIM to reclaim messages that have been pending
// (unacknowledged) for longer than ClaimMinIdleTime. This handles the case
// where a consumer crashed without acking.
func (q *Queue) claimPending(ctx context.Context, consumer string) {
	// XAUTOCLAIM claims pending messages starting from the minimum pending
	// ID. We use "0-0" to start from the beginning of the pending list.
	start := "0-0"
	for {
		msgs, next, err := q.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
			Stream:   q.cfg.Stream,
			Group:    q.cfg.Group,
			Consumer: consumer,
			MinIdle:  q.cfg.ClaimMinIdleTime,
			Start:    start,
			Count:    q.cfg.MaxPollCount,
		}).Result()

		if err != nil {
			if err != redis.Nil {
				logx.Errorf("redisstream: xautoclaim on stream %s: %v", q.cfg.Stream, err)
			}
			return
		}

		for _, msg := range msgs {
			q.processMessage(ctx, consumer, msg)
		}

		// If next is "0-0", there are no more pending messages to claim.
		if next == "0-0" || len(msgs) == 0 {
			return
		}
		start = next
	}
}

// processMessage calls the handler with the message and acks on success.
func (q *Queue) processMessage(ctx context.Context, consumer string, msg redis.XMessage) {
	data, ok := msg.Values["data"].(string)
	if !ok {
		logx.Errorf("redisstream: message %s on stream %s has no 'data' field", msg.ID, q.cfg.Stream)
		// Ack the malformed message so it doesn't block the stream.
		_ = q.client.XAck(ctx, q.cfg.Stream, q.cfg.Group, msg.ID).Err()
		return
	}

	// Call the handler. The key is the message ID (equivalent to Kafka
	// message key); the value is the JSON-encoded envelope.
	if err := q.handler.Consume(ctx, msg.ID, data); err != nil {
		logx.Errorf("redisstream: consume message %s on stream %s: %v", msg.ID, q.cfg.Stream, err)
		// Don't ack — the message stays pending and will be retried by
		// claimPending after ClaimMinIdleTime.
		return
	}

	// Acknowledge the message.
	if err := q.client.XAck(ctx, q.cfg.Stream, q.cfg.Group, msg.ID).Err(); err != nil {
		logx.Errorf("redisstream: xack message %s on stream %s: %v", msg.ID, q.cfg.Stream, err)
	}
}

// isBusyGroupError returns true if the error is "BUSYGROUP Consumer Group
// name already exists", which is expected when the group was created in a
// previous run.
func isBusyGroupError(err error) bool {
	return err != nil && (err.Error() == "BUSYGROUP Consumer Group name already exists")
}
