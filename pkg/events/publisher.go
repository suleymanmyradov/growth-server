package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
)

// Publisher pushes event envelopes to a Kafka topic via kq.Pusher.
type Publisher struct {
	pusher *kq.Pusher
	topic  string
}

// NewPublisher creates a Publisher that writes to the given topic on the
// specified broker list.
func NewPublisher(brokers []string, topic string) *Publisher {
	p := kq.NewPusher(brokers, topic)
	return &Publisher{pusher: p, topic: topic}
}

// Publish marshals the envelope to JSON and pushes it to Kafka. The call
// respects the context deadline; a 5-second timeout is applied if the
// caller's context has no deadline.
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
	if err := p.pusher.Push(ctx, string(data)); err != nil {
		return fmt.Errorf("push to topic %s: %w", p.topic, err)
	}
	return nil
}

// Close releases the underlying pusher resources.
func (p *Publisher) Close() error {
	return p.pusher.Close()
}
