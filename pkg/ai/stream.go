package ai

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/zeromicro/go-zero/core/logx"
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
		// Try fallback if configured for the initial stream setup.
		if sr, fbErr := c.tryFallbackStream(ctx, req, msgs, opts, streamErr, start); fbErr == nil {
			return sr, nil
		}
		latencyMS := time.Since(start).Milliseconds()
		c.logCall(ctx, req.ModelProfile, m.modelID, req.Metadata, Usage{}, latencyMS, 0, streamErr)
		recordMetrics(req.ModelProfile, m.modelID, "error", Usage{}, 0, latencyMS)
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

// tryFallbackStream attempts the fallback model for stream setup.
func (c *client) tryFallbackStream(ctx context.Context, req GenerateRequest, msgs []*schema.Message, opts []model.Option, primaryErr error, start time.Time) (StreamReader, error) {
	if !c.cfg.FallbackPolicy.Enabled {
		return nil, primaryErr
	}
	fb, ok := c.fallbackModel()
	if !ok {
		return nil, primaryErr
	}

	logx.WithContext(ctx).Infof("ai: primary stream failed, trying fallback: %v", primaryErr)

	var einoStream *schema.StreamReader[*schema.Message]
	streamErr := c.withRetry(ctx, fb.modelID, func(ctx context.Context) error {
		var genErr error
		einoStream, genErr = fb.chat.Stream(ctx, msgs, opts...)
		return genErr
	})
	if streamErr != nil {
		latencyMS := time.Since(start).Milliseconds()
		c.logCall(ctx, ModelFallback, fb.modelID, req.Metadata, Usage{}, latencyMS, 0, streamErr)
		recordMetrics(ModelFallback, fb.modelID, "error", Usage{}, 0, latencyMS)
		return nil, fmt.Errorf("ai.Stream fallback also failed: %w (primary: %v)", streamErr, primaryErr)
	}

	return &einoStreamReader{
		ctx:     ctx,
		stream:  einoStream,
		profile: ModelFallback,
		modelID: fb.modelID,
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
		latencyMS := time.Since(r.start).Milliseconds()
		if err == io.EOF {
			r.done.Store(true)
			costUSD := r.client.cfg.ComputeCost(r.modelID, r.total.PromptTokens, r.total.CompletionTokens)
			r.client.recordUsage(r.ctx, r.meta, r.total, costUSD)
			r.client.logCall(r.ctx, r.profile, r.modelID, r.meta, r.total, latencyMS, costUSD, nil)
			recordMetrics(r.profile, r.modelID, "ok", r.total, costUSD, latencyMS)
			return Chunk{FinishReason: "stop"}, io.EOF
		}
		r.client.logCall(r.ctx, r.profile, r.modelID, r.meta, r.total, latencyMS, 0, err)
		recordMetrics(r.profile, r.modelID, "error", r.total, 0, latencyMS)
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
