package ai

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/zeromicro/go-zero/core/logx"
)

// StreamAgent runs the model<->tool round-trip loop with streaming.
//
// The loop mirrors RunAgent but uses the streaming model API per step:
//   - Reasoning deltas (msg.ReasoningContent) are forwarded as Reasoning
//     events in real time. Only reasoning models emit these; for
//     non-reasoning models the field is always empty.
//   - Intermediate steps (model emits tool calls): tool calls are executed
//     and surfaced as ToolCall events. Content deltas emitted during
//     intermediate steps ARE forwarded in real time — with OpenAI-compatible
//     models, tool-call steps typically emit no content, but if the model
//     does emit reasoning text before a tool call, it reaches the caller.
//     However, only the FINAL step's content is included in the Complete
//     event's FullResponse (and thus in what the caller persists).
//   - Final step (model emits no tool calls): content deltas are forwarded
//     in real time as Delta events, followed by a Complete event with the
//     full response and cumulative usage.
//
// The caller reads events via Recv() until io.EOF. Close() cancels the
// loop and releases resources.
func (c *client) StreamAgent(ctx context.Context, req AgentRequest) (AgentStreamReader, error) {
	prep, err := c.prepareAgent(ctx, &req)
	if err != nil {
		return nil, err
	}

	// Bind tools once — the tooled model supports both Generate and Stream
	// (ToolCallingChatModel extends BaseChatModel).
	tooledModel, err := prep.m.chat.WithTools(prep.toolInfos)
	if err != nil {
		return nil, fmt.Errorf("ai.StreamAgent: bind tools: %w", err)
	}

	loopCtx, cancel := context.WithCancel(ctx)
	ch := make(chan AgentStreamChunk, 64)

	go c.runAgentStreamLoop(loopCtx, ch, agentStreamParams{
		tooledModel: tooledModel,
		toolInfos:   prep.toolInfos,
		modelID:     prep.m.modelID,
		profile:     req.ModelProfile,
		origProfile: req.ModelProfile,
		toolMap:     prep.toolMap,
		msgs:        prep.msgs,
		opts:        prep.opts,
		maxSteps:    req.MaxSteps,
		maxTokens:   req.MaxTotalTokens,
		meta:        req.Metadata,
	})

	return &agentStreamReader{ctx: loopCtx, cancel: cancel, ch: ch}, nil
}

type agentStreamParams struct {
	tooledModel model.ToolCallingChatModel
	toolInfos   []*schema.ToolInfo
	modelID     string
	profile     ModelProfile // current model's profile (updated on fallback)
	origProfile ModelProfile // originally requested profile (stable for fallback chain lookup)
	toolMap     map[string]Tool
	msgs        []*schema.Message
	opts        []model.Option
	maxSteps    int
	maxTokens   int
	meta        Metadata
}

const agentContinuationPrompt = "The previous assistant response was interrupted. Continue exactly where it stopped. Do not repeat any text, do not mention the interruption, and finish within 80 words."

// runAgentStreamLoop is the goroutine that drives the model<->tool loop and
// sends chunks to the channel. It closes the channel on exit.
//
// It intentionally does NOT receive or call the context's cancel func. The
// context is canceled only by the reader's Close() (for early termination).
// Closing the channel is sufficient to deliver io.EOF to a waiting Recv().
// If the goroutine also canceled the context, a Recv() select with both a
// buffered chunk and ctx.Done() ready would non-deterministically pick
// ctx.Done() and drop the buffered chunk — truncating the stream. Keeping
// cancellation under the reader's sole control eliminates that race.
func (c *client) runAgentStreamLoop(ctx context.Context, ch chan<- AgentStreamChunk, p agentStreamParams) {
	defer close(ch)

	start := time.Now()
	var totalUsage Usage
	var continuationPrefix strings.Builder
	continuationAttempts := 0

	// Captured thought_signatures from the previous step's tool calls.
	// Gemini requires these to be present on assistant tool_calls in the
	// conversation history; the transport injects them into the request.
	var prevSignatures map[string]string

	for step := 1; step <= p.maxSteps; step++ {
		// Per-step context: carry a fresh capture for the response, and
		// the previous step's signatures for request injection.
		capture := newThoughtSignatureCapture()
		stepCtx := withCapture(ctx, capture)
		if len(prevSignatures) > 0 {
			stepCtx = withInject(stepCtx, prevSignatures)
		}

		// Open a streaming call with tools bound, falling back through
		// c.fallbackModelsFor(p.origProfile) if the primary fails to open
		// (e.g. provider 503/429). On a successful fallback, p is rebound
		// in place so subsequent steps and final metrics/usage attribute
		// to the fallback model. This mirrors tryFallbackStream for the
		// non-agent Stream path — without it, a single 503 from the
		// primary model kills the whole agentic turn.
		einoStream, err := c.openAgentStream(stepCtx, &p, step)
		if err != nil {
			c.finishAgentStreamError(ctx, ch, p, start, totalUsage, fmt.Errorf("ai.StreamAgent step %d: stream open: %w", step, err))
			return
		}

		// Read the stream, forwarding content deltas in real time and
		// accumulating tool calls. With OpenAI-compatible models, a step
		// typically emits either content (final answer) or tool calls
		// (intermediate), rarely both. When both appear, the content is
		// the model's reasoning before the tool call — we forward it so
		// the caller can show it if desired, and it becomes part of the
		// conversation history.
		var contentBuf strings.Builder
		var toolCalls []schema.ToolCall
		var stepUsage Usage
		var finishReason string
		thoughtFilt := newThoughtFilter()

		for {
			msg, recvErr := einoStream.Recv()
			if recvErr != nil {
				if recvErr == io.EOF {
					break
				}
				einoStream.Close()
				c.finishAgentStreamError(ctx, ch, p, start, totalUsage, fmt.Errorf("ai.StreamAgent step %d: recv: %w", step, recvErr))
				return
			}

			if msg.ReasoningContent != "" {
				// Forward reasoning/thinking delta to caller in real time.
				// Only reasoning models emit this; for non-reasoning models
				// it is always empty. The caller can surface it as the
				// model's live "thinking" process.
				select {
				case ch <- AgentStreamChunk{Reasoning: msg.ReasoningContent}:
				case <-ctx.Done():
					einoStream.Close()
					return
				}
			}
			if msg.Content != "" {
				// Split <thought> tags from Gemma-4 models: thought content
				// → Reasoning (surfaced as the model's live thinking process),
				// rest → Delta (the actual response). The filter is stateful
				// across deltas within this step.
				parts := thoughtFilt.filter(msg.Content)
				contentBuf.WriteString(parts.Content)
				if parts.Reasoning != "" {
					select {
					case ch <- AgentStreamChunk{Reasoning: parts.Reasoning}:
					case <-ctx.Done():
						einoStream.Close()
						return
					}
				}
				if parts.Content != "" {
					// Forward content delta to caller in real time.
					// NOTE: this forwards content from ALL steps, including
					// intermediate ones. fullResponse (used for the Complete
					// event) is reset below for intermediate steps so only
					// the final answer is persisted.
					select {
					case ch <- AgentStreamChunk{Delta: parts.Content}:
					case <-ctx.Done():
						einoStream.Close()
						return
					}
				}
			}
			if len(msg.ToolCalls) > 0 {
				toolCalls = append(toolCalls, msg.ToolCalls...)
			}
			if msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
				u := msg.ResponseMeta.Usage
				stepUsage = Usage{
					PromptTokens:     u.PromptTokens,
					CompletionTokens: u.CompletionTokens,
					TotalTokens:      u.TotalTokens,
				}
			}
			// Capture the provider's finish_reason for this step. On the
			// final step (no tool calls), this is surfaced on the Complete
			// chunk so the caller can detect truncation (finish_reason=
			// "length" means the provider capped the output at max_tokens
			// and FullResponse is incomplete).
			if msg.ResponseMeta != nil && msg.ResponseMeta.FinishReason != "" {
				finishReason = msg.ResponseMeta.FinishReason
			}
		}
		einoStream.Close()

		// Flush the thought filter — if the stream ended while still inside a
		// <thought> block (truncated), the buffered content is routed as
		// reasoning. If there's remaining non-thought content, forward it as delta.
		if remaining := thoughtFilt.flush(); remaining.Content != "" || remaining.Reasoning != "" {
			contentBuf.WriteString(remaining.Content)
			if remaining.Reasoning != "" {
				select {
				case ch <- AgentStreamChunk{Reasoning: remaining.Reasoning}:
				case <-ctx.Done():
					return
				}
			}
			if remaining.Content != "" {
				select {
				case ch <- AgentStreamChunk{Delta: remaining.Content}:
				case <-ctx.Done():
					return
				}
			}
		}

		accumulateUsage(&totalUsage, stepUsage)

		// Enforce cumulative token budget.
		if p.maxTokens > 0 && totalUsage.TotalTokens > p.maxTokens {
			c.finishAgentStreamError(ctx, ch, p, start, totalUsage,
				fmt.Errorf("ai.StreamAgent: max total tokens exceeded (%d > %d): %w", totalUsage.TotalTokens, p.maxTokens, ErrMaxTokens))
			return
		}

		// Build the assistant message for the conversation history.
		// Preserve captured thought_signatures on the tool calls so the
		// transport can inject them into the next request for Gemini.
		assistantMsg := &schema.Message{
			Role:    schema.Assistant,
			Content: contentBuf.String(),
		}
		if len(toolCalls) > 0 {
			assistantMsg.ToolCalls = toolCalls
			// Merge this step's signatures into prevSignatures rather than
			// replacing them. The conversation history accumulates tool
			// calls from ALL steps, and Gemini 3.x requires every
			// assistant tool_call in the current turn to carry its
			// thought_signature. If we replace instead of merge, earlier
			// steps' tool calls lose their signatures and the next
			// request fails with 400 "Function call is missing a
			// thought_signature in functionCall parts".
			if prevSignatures == nil {
				prevSignatures = make(map[string]string)
			}
			for k, v := range capture.all() {
				prevSignatures[k] = v
			}
		} else {
			prevSignatures = nil
		}
		p.msgs = append(p.msgs, assistantMsg)

		// No tool calls → this is the final answer. Emit Complete with
		// this step's content only after the provider explicitly confirms
		// normal completion. A clean transport EOF alone is not enough:
		// providers can close a partial response without a finish reason.
		// Do NOT apply this check to tool-call steps — tools must be
		// executed whenever they are present, regardless of finish_reason.
		if len(toolCalls) == 0 {
			if !strings.EqualFold(finishReason, "stop") {
				if contentBuf.Len() > 0 && continuationAttempts == 0 && step < p.maxSteps && isRecoverableIncompleteFinish(finishReason) {
					continuationPrefix.WriteString(contentBuf.String())
					p.msgs = append(p.msgs, &schema.Message{
						Role:    schema.User,
						Content: agentContinuationPrompt,
					})
					continuationAttempts++
					logx.WithContext(ctx).Infof("ai.StreamAgent step %d: incomplete stream with finish_reason %q; attempting continuation", step, finishReason)
					continue
				}
				c.finishAgentStreamError(ctx, ch, p, start, totalUsage,
					fmt.Errorf("ai.StreamAgent step %d: stream ended with finish_reason %q: %w", step, finishReason, ErrStreamIncomplete))
				return
			}
			c.finishAgentStreamOK(ctx, ch, p, start, totalUsage, continuationPrefix.String()+contentBuf.String(), finishReason)
			return
		}

		// Tool calls present → this is an intermediate step. Content
		// deltas from this step (if any) were already forwarded to the
		// caller in real time; they are not included in the Complete
		// event's FullResponse (only the final step's content is).
		// For each tool call, emit a "started" event before execution
		// (so the caller can show a status message immediately) and a
		// "completed" event after execution (carrying the result).
		for _, tc := range toolCalls {
			// Emit "started" before execution so the caller can show
			// a "Looking up..." status while the tool runs.
			select {
			case ch <- AgentStreamChunk{ToolCall: &ToolCallEvent{
				Step:   step,
				Name:   tc.Function.Name,
				Status: ToolStatusStarted,
				Args:   tc.Function.Arguments,
			}}:
			case <-ctx.Done():
				return
			}

			execRes := executeToolCall(ctx, p.toolMap, tc.Function.Name, tc.Function.Arguments, "ai.StreamAgent")

			select {
			case ch <- AgentStreamChunk{ToolCall: &ToolCallEvent{
				Step:   step,
				Name:   tc.Function.Name,
				Status: ToolStatusCompleted,
				Args:   tc.Function.Arguments,
				Result: execRes.Output,
				Error:  execRes.ErrMsg,
			}}:
			case <-ctx.Done():
				return
			}

			p.msgs = append(p.msgs, toEinoMessage(toolResultMessage(tc.ID, execRes.Output)))
		}
	}

	// Hit max steps without a final answer.
	logx.WithContext(ctx).Infof("ai.StreamAgent: hit max steps %d", p.maxSteps)
	c.finishAgentStreamError(ctx, ch, p, start, totalUsage, fmt.Errorf("ai.StreamAgent: %w (max %d steps)", ErrMaxSteps, p.maxSteps))
}

func isRecoverableIncompleteFinish(finishReason string) bool {
	return finishReason == "" || strings.EqualFold(finishReason, "length")
}

// openAgentStream opens a streaming tool-call request against the primary
// tooled model, falling back through c.fallbackModelsFor(p.origProfile) if
// the primary fails to open the stream (e.g. provider 503/429 "high demand").
// On a successful fallback, p.tooledModel, p.modelID, and p.profile are
// updated in place so subsequent steps and final metrics/usage log against
// the fallback model. The fallback chain is always resolved from the
// originally requested profile (origProfile), and the model currently in use
// is skipped so we never retry the model that just failed.
//
// This mirrors tryFallbackStream for the non-agent Stream path. Without it,
// StreamAgent has no fallback and a single 503 from the primary model kills
// the whole agentic turn even when a fallback chain is configured.
func (c *client) openAgentStream(ctx context.Context, p *agentStreamParams, step int) (*schema.StreamReader[*schema.Message], error) {
	einoStream, err := p.tooledModel.Stream(ctx, p.msgs, p.opts...)
	if err == nil {
		return einoStream, nil
	}
	if !c.cfg.FallbackPolicy.Enabled {
		return nil, err
	}
	chain := c.fallbackModelsFor(p.origProfile)
	if len(chain) == 0 {
		return nil, err
	}

	logx.WithContext(ctx).Infof("ai.StreamAgent step %d: primary model %s stream open failed, trying %d fallback(s): %v", step, p.modelID, len(chain), err)

	lastErr := err
	for i, fb := range chain {
		if fb.modelID == p.modelID {
			continue // never retry the model that just failed
		}
		logx.WithContext(ctx).Infof("ai.StreamAgent step %d: trying stream fallback %d/%d: %s", step, i+1, len(chain), fb.modelID)

		fbTooled, bindErr := fb.chat.WithTools(p.toolInfos)
		if bindErr != nil {
			lastErr = fmt.Errorf("bind tools on fallback %s: %w", fb.modelID, bindErr)
			logx.WithContext(ctx).Infof("ai.StreamAgent step %d: fallback %d/%d (%s) bind tools failed: %v", step, i+1, len(chain), fb.modelID, bindErr)
			continue
		}

		fbStream, fbErr := fbTooled.Stream(ctx, p.msgs, p.opts...)
		if fbErr != nil {
			lastErr = fbErr
			logx.WithContext(ctx).Infof("ai.StreamAgent step %d: fallback %d/%d (%s) stream open failed: %v", step, i+1, len(chain), fb.modelID, fbErr)
			continue
		}

		// Fallback succeeded — rebind the loop to this model for subsequent
		// steps so the rest of the turn and final usage/metrics attribute
		// to the fallback model.
		p.tooledModel = fbTooled
		p.modelID = fb.modelID
		p.profile = ModelFallback
		logx.WithContext(ctx).Infof("ai.StreamAgent step %d: fallback %d/%d (%s) stream opened", step, i+1, len(chain), fb.modelID)
		return fbStream, nil
	}

	return nil, fmt.Errorf("ai.StreamAgent step %d: all %d fallback(s) failed: %w (primary: %v)", step, len(chain), lastErr, err)
}

// finishAgentStreamOK sends the Complete event and records metrics/usage.
func (c *client) finishAgentStreamOK(ctx context.Context, ch chan<- AgentStreamChunk, p agentStreamParams, start time.Time, totalUsage Usage, fullResponse string, finishReason string) {
	costUSD := c.cfg.ComputeCost(p.modelID, totalUsage.PromptTokens, totalUsage.CompletionTokens)
	latencyMS := time.Since(start).Milliseconds()

	c.recordUsage(ctx, p.meta, totalUsage, costUSD)
	c.logCall(ctx, p.profile, p.modelID, p.meta, totalUsage, latencyMS, costUSD, nil)
	recordMetrics(p.profile, p.modelID, "ok", p.meta.Feature, totalUsage, costUSD, latencyMS)

	usage := totalUsage
	select {
	case ch <- AgentStreamChunk{
		Complete:     true,
		FullResponse: fullResponse,
		FinishReason: finishReason,
		Usage:        &usage,
	}:
	case <-ctx.Done():
	}
}

// finishAgentStreamError sends an Error event, records metrics/usage, and
// returns. The caller (goroutine) will close the channel.
func (c *client) finishAgentStreamError(ctx context.Context, ch chan<- AgentStreamChunk, p agentStreamParams, start time.Time, totalUsage Usage, err error) {
	latencyMS := time.Since(start).Milliseconds()
	costUSD := c.cfg.ComputeCost(p.modelID, totalUsage.PromptTokens, totalUsage.CompletionTokens)

	c.logCall(ctx, p.profile, p.modelID, p.meta, totalUsage, latencyMS, costUSD, err)
	recordMetrics(p.profile, p.modelID, "error", p.meta.Feature, totalUsage, costUSD, latencyMS)

	select {
	case ch <- AgentStreamChunk{Error: err}:
	case <-ctx.Done():
	}
}

// agentStreamReader implements AgentStreamReader by reading from a channel.
type agentStreamReader struct {
	ctx    context.Context
	cancel context.CancelFunc
	ch     <-chan AgentStreamChunk
	close  sync.Once
}

func (r *agentStreamReader) Recv() (AgentStreamChunk, error) {
	select {
	case chunk, ok := <-r.ch:
		if !ok {
			return AgentStreamChunk{}, io.EOF
		}
		if chunk.Error != nil {
			return chunk, chunk.Error
		}
		return chunk, nil
	case <-r.ctx.Done():
		return AgentStreamChunk{}, io.EOF
	}
}

func (r *agentStreamReader) Close() {
	r.close.Do(func() {
		r.cancel()
		// Drain the channel to unblock the goroutine if it's blocked on send.
		for range r.ch {
		}
	})
}

// Ensure agentStreamReader implements AgentStreamReader.
var _ AgentStreamReader = (*agentStreamReader)(nil)
