package ai

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"time"
	"unicode/utf8"

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
	streamErr := c.withRetry(ctx, m.modelID, func(attemptCtx context.Context) error {
		var genErr error
		// Use the long-lived ctx, not attemptCtx, to open the stream. withRetry
		// cancels attemptCtx via defer cancel() when this function returns, which
		// would close the HTTP connection to OpenRouter before any chunks are
		// read. The stream reader needs the connection to stay open for the
		// entire generation, so it must be tied to ctx (which lives until the
		// caller is done with the stream).
		einoStream, genErr = m.chat.Stream(ctx, msgs, opts...)
		if genErr != nil {
			logx.WithContext(ctx).Errorf("ai.Stream: model %s stream open failed: %v (type=%T), ctx.Err()=%v", m.modelID, genErr, genErr, ctx.Err())
		}
		return genErr
	})
	if streamErr != nil {
		// Try fallback if configured for the initial stream setup.
		if sr, fbErr := c.tryFallbackStream(ctx, req, msgs, opts, streamErr, start); fbErr == nil {
			return sr, nil
		}
		latencyMS := time.Since(start).Milliseconds()
		c.logCall(ctx, req.ModelProfile, m.modelID, req.Metadata, Usage{}, latencyMS, 0, streamErr)
		recordMetrics(req.ModelProfile, m.modelID, "error", req.Metadata.Feature, Usage{}, 0, latencyMS)
		return nil, fmt.Errorf("ai.Stream: %w", streamErr)
	}

	logx.WithContext(ctx).Infof("ai.Stream: model %s stream opened after %v", m.modelID, time.Since(start))

	return &einoStreamReader{
		ctx:           ctx,
		stream:        einoStream,
		profile:       req.ModelProfile,
		modelID:       m.modelID,
		meta:          req.Metadata,
		client:        c,
		start:         start,
		thoughtFilter: newThoughtFilter(),
	}, nil
}

// tryFallbackStream attempts fallback models in chain order for stream setup.
// It tries each cross-provider fallback (in FallbackProviders list order) and
// finally the same-provider ModelFallback profile, until one successfully
// opens a stream.
func (c *client) tryFallbackStream(ctx context.Context, req GenerateRequest, msgs []*schema.Message, opts []model.Option, primaryErr error, start time.Time) (StreamReader, error) {
	if !c.cfg.FallbackPolicy.Enabled {
		return nil, primaryErr
	}
	chain := c.fallbackModelsFor(req.ModelProfile)
	if len(chain) == 0 {
		return nil, primaryErr
	}

	logx.WithContext(ctx).Infof("ai: primary stream %s failed, trying %d fallback(s): %v", req.ModelProfile, len(chain), primaryErr)

	var lastErr = primaryErr

	for i, fb := range chain {
		logx.WithContext(ctx).Infof("ai: trying stream fallback %d/%d: %s", i+1, len(chain), fb.modelID)

		var einoStream *schema.StreamReader[*schema.Message]
		streamErr := c.withRetry(ctx, fb.modelID, func(attemptCtx context.Context) error {
			var genErr error
			// Use ctx, not attemptCtx — see Stream() for why the stream connection
			// must outlive the per-attempt context.
			einoStream, genErr = fb.chat.Stream(ctx, msgs, opts...)
			return genErr
		})
		if streamErr != nil {
			lastErr = streamErr
			latencyMS := time.Since(start).Milliseconds()
			c.logCall(ctx, ModelFallback, fb.modelID, req.Metadata, Usage{}, latencyMS, 0, streamErr)
			recordMetrics(ModelFallback, fb.modelID, "error", req.Metadata.Feature, Usage{}, 0, latencyMS)
			logx.WithContext(ctx).Infof("ai: stream fallback %d/%d (%s) also failed: %v", i+1, len(chain), fb.modelID, streamErr)
			continue
		}

		return &einoStreamReader{
			ctx:           ctx,
			stream:        einoStream,
			profile:       ModelFallback,
			modelID:       fb.modelID,
			meta:          req.Metadata,
			client:        c,
			start:         start,
			thoughtFilter: newThoughtFilter(),
		}, nil
	}

	return nil, fmt.Errorf("ai.Stream all %d fallback(s) failed: %w (primary: %v)", len(chain), lastErr, primaryErr)
}

// einoStreamReader adapts Eino's StreamReader[*schema.Message] to our StreamReader.
type einoStreamReader struct {
	ctx           context.Context
	stream        *schema.StreamReader[*schema.Message]
	profile       ModelProfile
	modelID       string
	meta          Metadata
	client        *client
	start         time.Time
	total         Usage
	done          atomic.Bool
	thoughtFilter *thoughtFilter
	pendingFlush  thoughtParts // content from thoughtFilter.flush() to return before EOF
}

// Recv returns the next Chunk from the stream.
func (r *einoStreamReader) Recv() (Chunk, error) {
	if r.done.Load() {
		return Chunk{}, io.EOF
	}

	// If we have pending flush content (from thoughtFilter), return it first
	// before delivering the final EOF.
	if r.pendingFlush.Content != "" || r.pendingFlush.Reasoning != "" {
		flush := r.pendingFlush
		r.pendingFlush = thoughtParts{}
		r.done.Store(true)
		costUSD := r.client.cfg.ComputeCost(r.modelID, r.total.PromptTokens, r.total.CompletionTokens)
		r.client.recordUsage(r.ctx, r.meta, r.total, costUSD)
		r.client.logCall(r.ctx, r.profile, r.modelID, r.meta, r.total, time.Since(r.start).Milliseconds(), costUSD, nil)
		recordMetrics(r.profile, r.modelID, "ok", r.meta.Feature, r.total, costUSD, time.Since(r.start).Milliseconds())
		return Chunk{Delta: flush.Content, Reasoning: flush.Reasoning, FinishReason: "stop"}, io.EOF
	}

	msg, err := r.stream.Recv()
	if err != nil {
		latencyMS := time.Since(r.start).Milliseconds()
		if err == io.EOF {
			// Flush the thought filter — if there's remaining content or
			// reasoning, return it before the stop chunk.
			remaining := r.thoughtFilter.flush()
			if remaining.Content != "" || remaining.Reasoning != "" {
				r.pendingFlush = remaining
				return Chunk{Delta: remaining.Content, Reasoning: remaining.Reasoning}, nil
			}
			r.done.Store(true)
			costUSD := r.client.cfg.ComputeCost(r.modelID, r.total.PromptTokens, r.total.CompletionTokens)
			r.client.recordUsage(r.ctx, r.meta, r.total, costUSD)
			r.client.logCall(r.ctx, r.profile, r.modelID, r.meta, r.total, latencyMS, costUSD, nil)
			recordMetrics(r.profile, r.modelID, "ok", r.meta.Feature, r.total, costUSD, latencyMS)
			return Chunk{FinishReason: "stop"}, io.EOF
		}
		// Log detailed error: error type, context state, tokens received so far.
		ctxErr := r.ctx.Err()
		logx.WithContext(r.ctx).Errorf("ai.Stream recv error: err=%v (type=%T), model=%s, ctx.Err()=%v, latency=%dms, promptTokens=%d, completionTokens=%d",
			err, err, r.modelID, ctxErr, latencyMS, r.total.PromptTokens, r.total.CompletionTokens)
		r.client.logCall(r.ctx, r.profile, r.modelID, r.meta, r.total, latencyMS, 0, err)
		recordMetrics(r.profile, r.modelID, "error", r.meta.Feature, r.total, 0, latencyMS)
		return Chunk{}, err
	}

	// Sanitize at the source: some LLM streaming APIs (especially free-tier
	// models on OpenRouter) emit invalid UTF-8 sequences that would cause
	// gRPC marshaling errors downstream. Then split <thought> tags from
	// Gemma-4 models: thought content → Reasoning, rest → Delta.
	parts := r.thoughtFilter.filter(sanitizeUTF8(msg.Content))
	chunk := Chunk{
		Delta:     parts.Content,
		Reasoning: parts.Reasoning,
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

// sanitizeUTF8 ensures a string is valid UTF-8 by replacing invalid bytes with
// the Unicode replacement character. Some LLM streaming APIs (especially
// free-tier models on OpenRouter) emit invalid sequences that would cause gRPC
// marshaling errors downstream.
func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	v := make([]rune, 0, len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			v = append(v, 0xFFFD)
		} else {
			v = append(v, r)
		}
		i += size
	}
	return string(v)
}

// Suppress unused import.
var _ = model.WithTemperature
