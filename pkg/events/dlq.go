package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-queue/kq"
)

// DLQTopic is the default dead-letter topic/stream name.
const DLQTopic = "growth.events.dlq"

// DLQMessage wraps a failed event with metadata about why it was rejected.
type DLQMessage struct {
	Original    Envelope  `json:"original,omitempty"`
	Raw         string    `json:"raw,omitempty"`
	Reason      string    `json:"reason"`
	Permanent   bool      `json:"permanent"`
	ServiceName string    `json:"serviceName"`
	OccurredAt  time.Time `json:"occurredAt"`
}

// DLQPublisher pushes poison messages to a dead-letter stream (Kafka topic
// or Redis Stream).
type DLQPublisher struct {
	pusher      *kq.Pusher    // Kafka backend (nil when using Redis Streams)
	redisClient *redis.Client // Redis backend (nil when using Kafka)
	topic       string
}

// NewDLQPublisher creates a DLQ publisher that writes to the given Kafka topic.
func NewDLQPublisher(brokers []string, topic string) *DLQPublisher {
	p := kq.NewPusher(brokers, topic)
	return &DLQPublisher{pusher: p, topic: topic}
}

// NewRedisStreamDLQPublisher creates a DLQ publisher that writes to a Redis
// Stream with the given name.
func NewRedisStreamDLQPublisher(client *redis.Client, stream string) *DLQPublisher {
	return &DLQPublisher{redisClient: client, topic: stream}
}

// Publish marshals the DLQ message to JSON and pushes it to the configured backend.
func (p *DLQPublisher) Publish(ctx context.Context, msg DLQMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal dlq message: %w", err)
	}

	if p.redisClient != nil {
		if err := p.redisClient.XAdd(ctx, &redis.XAddArgs{
			Stream: p.topic,
			MaxLen: 10000,
			Approx: true,
			Values: map[string]interface{}{"data": string(data)},
		}).Err(); err != nil {
			return fmt.Errorf("xadd to dlq stream %s: %w", p.topic, err)
		}
		return nil
	}

	if err := p.pusher.Push(ctx, string(data)); err != nil {
		return fmt.Errorf("push to dlq topic %s: %w", p.topic, err)
	}
	return nil
}

// Close releases the underlying backend resources.
func (p *DLQPublisher) Close() error {
	if p.redisClient != nil {
		return nil
	}
	return p.pusher.Close()
}
