// Package userdeletion provides a reusable consumer for user_deleted events.
// Any service that owns user-keyed tables wires its cleanup callback into the
// Handler and gets a queue.MessageQueue over Kafka (brokers configured) or
// Redis Streams (fallback), matching the transport pattern used by the client
// service's consumers.
package userdeletion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/pkg/events/redisstream"
	"github.com/suleymanmyradov/growth-server/pkg/redisutil"
	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/queue"
)

// Config selects the transport for the user_deleted consumer. Topic empty
// disables the consumer entirely; otherwise Group is required and exactly one
// transport must be reachable (Kafka brokers or Redis).
type Config struct {
	Brokers       []string `json:",optional"`
	Topic         string   `json:",optional"`
	Group         string   `json:",optional"`
	RedisAddr     string   `json:",optional"`
	RedisPassword string   `json:",optional" secret:"true"`
	RedisDB       int      `json:",optional"`
}

// Handler deletes the user's data via the Delete callback. The callback must
// be idempotent: redelivery calls it again.
type Handler struct {
	Delete func(context.Context, uuid.UUID) error
}

// Consume parses the envelope and invokes Delete for valid user_deleted
// events. Malformed envelopes, unrelated event types, unsupported versions,
// and invalid IDs are logged and ignored — never deleted on. A Delete error
// is returned so the queue redelivers.
func (h Handler) Consume(ctx context.Context, _ string, raw string) error {
	var env events.Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		logx.WithContext(ctx).Errorf("userdeletion: invalid envelope: %v", err)
		return nil
	}
	if events.EventType(env.EventType) != events.TypeUserDeleted {
		return nil
	}
	if env.Version != 1 {
		logx.WithContext(ctx).Errorf("userdeletion: unsupported version %d, ignoring", env.Version)
		return nil
	}
	eventID, err := uuid.Parse(env.EventID)
	if err != nil || eventID == uuid.Nil {
		logx.WithContext(ctx).Errorf("userdeletion: invalid event ID %q, ignoring", env.EventID)
		return nil
	}

	var p events.UserDeleted
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		logx.WithContext(ctx).Errorf("userdeletion: invalid payload: %v", err)
		return nil
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil || userID == uuid.Nil {
		logx.WithContext(ctx).Errorf("userdeletion: invalid user ID in event %s, ignoring", env.EventID)
		return nil
	}

	return h.Delete(ctx, userID)
}

// NewQueue builds the consumer queue for the configured transport. It returns
// (nil, close, nil) when disabled, an error when the config is incomplete, and
// a close func that releases resources owned by the queue (the Redis client
// on the fallback transport). The caller must Stop the queue before invoking
// the returned close func.
func NewQueue(c Config, handler Handler) (queue.MessageQueue, func(), error) {
	noop := func() {}
	if c.Topic == "" {
		return nil, noop, nil
	}
	if c.Group == "" {
		return nil, noop, errors.New("userdeletion: Group is required when Topic is set")
	}
	if len(c.Brokers) > 0 {
		q, err := kq.NewQueue(
			kq.KqConf{
				Brokers:    c.Brokers,
				Group:      c.Group,
				Topic:      c.Topic,
				Offset:     "first",
				Consumers:  1,
				Processors: 1,
			},
			kq.WithHandle(handler.Consume),
		)
		if err != nil {
			return nil, noop, fmt.Errorf("userdeletion: kafka queue: %w", err)
		}
		return q, noop, nil
	}
	if c.RedisAddr != "" {
		client, err := redisutil.NewClient(c.RedisAddr, c.RedisPassword, c.RedisDB)
		if err != nil {
			return nil, noop, fmt.Errorf("userdeletion: redis client: %w", err)
		}
		q := redisstream.NewQueue(client, redisstream.Config{
			Stream:    c.Topic,
			Group:     c.Group,
			Consumers: 1,
		}, handler)
		return q, func() { _ = client.Close() }, nil
	}
	return nil, noop, errors.New("userdeletion: Topic set but no transport configured (Brokers or RedisAddr)")
}
