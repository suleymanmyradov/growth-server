package ai

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Stream performs streaming generation.
func (c *client) Stream(ctx context.Context, req GenerateRequest) (StreamReader, error) {
	if err := c.checkQuota(ctx, req.Metadata); err != nil {
		return nil, err
	}

	m, err := c.modelFor(req.ModelProfile)
	if err != nil {
		return nil, err
	}

	msgs := toEinoMessages(req.Messages, req.System)
	opts := einoModelOptions(req)

	start := time.Now()

	var einoStream *schema.StreamReader[*schema.Message]
	streamErr := c.withRetry(ctx, m.modelID, func(ctx context.Context) error {
		var genErr error
		einoStream, genErr = m.chat.Stream(ctx, msgs, opts...)
		return genErr
	})
	if streamErr != nil {
		c.logCall(ctx, req.ModelProfile, m.modelID, req.Metadata, Usage{}, time.Since(start).Milliseconds(), 0, streamErr)
		recordMetrics(req.ModelProfile, m.modelID, "error")
		return nil, fmt.Errorf("ai.Stream: %w", streamErr)
	}

	return &einoStreamReader{
		ctx:     ctx,
		stream:  einoStream,
		profile: req.ModelProfile,
		modelID: m.modelID,
		meta:    req.Metadata,
		client:  c,
		start:   start,
	}, nil
}

// einoStreamReader adapts Eino's StreamReader[*schema.Message] to our StreamReader.
type einoStreamReader struct {
	ctx     context.Context
	stream  *schema.StreamReader[*schema.Message]
	profile ModelProfile
	modelID string
	meta    Metadata
	client  *client
	start   time.Time
	total   Usage
	done    atomic.Bool
}

// Recv returns the next Chunk from the stream.
func (r *einoStreamReader) Recv() (Chunk, error) {
	if r.done.Load() {
		return Chunk{}, io.EOF
	}

	msg, err := r.stream.Recv()
	if err != nil {
		if err == io.EOF {
			r.done.Store(true)
			r.client.logCall(r.ctx, r.profile, r.modelID, r.meta, r.total, time.Since(r.start).Milliseconds(), 0, nil)
			recordMetrics(r.profile, r.modelID, "ok")
			return Chunk{FinishReason: "stop"}, io.EOF
		}
		r.client.logCall(r.ctx, r.profile, r.modelID, r.meta, r.total, time.Since(r.start).Milliseconds(), 0, err)
		recordMetrics(r.profile, r.modelID, "error")
		return Chunk{}, err
	}

	chunk := Chunk{
		Delta: msg.Content,
	}

	// Extract tool call deltas if present.
	if len(msg.ToolCalls) > 0 {
		tc := msg.ToolCalls[0]
		chunk.ToolCallDelta = &ToolCallDelta{
			ID:     tc.ID,
			FnName: tc.Function.Name,
			FnArgs: tc.Function.Arguments,
		}
		if tc.Index != nil {
			chunk.ToolCallDelta.Index = *tc.Index
		}
	}

	// Extract usage from the final message.
	if msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
		u := msg.ResponseMeta.Usage
		r.total = Usage{
			PromptTokens:     u.PromptTokens,
			CompletionTokens: u.CompletionTokens,
			TotalTokens:      u.TotalTokens,
		}
		chunk.Usage = &r.total
		if msg.ResponseMeta.FinishReason != "" {
			chunk.FinishReason = msg.ResponseMeta.FinishReason
		}
	}

	return chunk, nil
}

// Close releases the underlying stream.
func (r *einoStreamReader) Close() {
	r.stream.Close()
	r.done.Store(true)
}

// Ensure einoStreamReader implements StreamReader.
var _ StreamReader = (*einoStreamReader)(nil)

// Suppress unused import.
var _ = model.WithTemperature
