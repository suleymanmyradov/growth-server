package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
)

// Publisher pushes event envelopes to a message stream — either a Kafka topic
// (via kq.Pusher) or a Redis Stream (via go-redis XAdd). The backend is
// chosen at construction time; Publish/Close are backend-agnostic.
type Publisher struct {
	pusher      *kq.Pusher    // Kafka backend (nil when using Redis Streams)
	redisClient *redis.Client // Redis backend (nil when using Kafka)
	topic       string
}

// NewPublisher creates a Publisher that writes to the given Kafka topic on
// the specified broker list.
func NewPublisher(brokers []string, topic string) *Publisher {
	p := kq.NewPusher(brokers, topic)
	return &Publisher{pusher: p, topic: topic}
}

// NewRedisStreamPublisher creates a Publisher that writes to a Redis Stream
// with the given name. This is the Redis Streams alternative to Kafka — use
// it when Kafka brokers are not available but Redis is.
func NewRedisStreamPublisher(client *redis.Client, stream string) *Publisher {
	return &Publisher{redisClient: client, topic: stream}
}

// Publish marshals the envelope to JSON and pushes it to the configured
// backend (Kafka or Redis Streams). The call respects the context deadline;
// a 5-second timeout is applied if the caller's context has no deadline.
func (p *Publisher) Publish(ctx context.Context, env Envelope) error {
	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before publish: %w", err)
	}

	logx.WithContext(ctx).Infof("publishing event %s to topic %s", env.EventID, p.topic)

	if p.redisClient != nil {
		// Redis Streams backend: XADD with MAXLEN trimming to prevent
		// unbounded stream growth. ~100k entries is plenty for event
		// replay; consumers read via consumer groups so they never miss
		// messages they haven't acknowledged yet.
		if err := p.redisClient.XAdd(ctx, &redis.XAddArgs{
			Stream: p.topic,
			MaxLen: 100000,
			Approx: true,
			Values: map[string]interface{}{"data": string(data)},
		}).Err(); err != nil {
			return fmt.Errorf("xadd to stream %s: %w", p.topic, err)
		}
		return nil
	}

	// Kafka backend.
	if err := p.pusher.Push(ctx, string(data)); err != nil {
		return fmt.Errorf("push to topic %s: %w", p.topic, err)
	}
	return nil
}

// Close releases the underlying backend resources.
func (p *Publisher) Close() error {
	if p.redisClient != nil {
		// Redis client lifecycle is owned by the service context, not the
		// publisher — don't close it here.
		return nil
	}
	return p.pusher.Close()
}
