package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zeromicro/go-queue/kq"
)

// DLQTopic is the default dead-letter topic name.
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

// DLQPublisher pushes poison messages to a dead-letter topic.
type DLQPublisher struct {
	pusher *kq.Pusher
	topic  string
}

// NewDLQPublisher creates a DLQ publisher that writes to the given topic.
func NewDLQPublisher(brokers []string, topic string) *DLQPublisher {
	p := kq.NewPusher(brokers, topic)
	return &DLQPublisher{pusher: p, topic: topic}
}

// Publish marshals the DLQ message to JSON and pushes it to Kafka.
func (p *DLQPublisher) Publish(ctx context.Context, msg DLQMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal dlq message: %w", err)
	}
	if err := p.pusher.Push(ctx, string(data)); err != nil {
		return fmt.Errorf("push to dlq topic %s: %w", p.topic, err)
	}
	return nil
}

// Close releases the underlying pusher resources.
func (p *DLQPublisher) Close() error {
	return p.pusher.Close()
}
