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
	if len(req.Tools) == 0 {
		return nil, ErrNoTools
	}
	if req.MaxSteps <= 0 {
		req.MaxSteps = 10
	}

	if err := c.checkQuota(ctx, req.Metadata); err != nil {
		return nil, err
	}

	m, err := c.modelFor(req.ModelProfile)
	if err != nil {
		return nil, err
	}

	toolInfos := buildToolInfos(req.Tools)
	toolMap := make(map[string]Tool, len(req.Tools))
	for _, t := range req.Tools {
		toolMap[t.Name()] = t
	}

	msgs := toEinoMessages(req.Messages, req.System)
	opts := einoModelOptions(GenerateRequest{
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	})

	// Bind tools once — the tooled model supports both Generate and Stream
	// (ToolCallingChatModel extends BaseChatModel).
	tooledModel, err := m.chat.WithTools(toolInfos)
	if err != nil {
		return nil, fmt.Errorf("ai.StreamAgent: bind tools: %w", err)
	}

	loopCtx, cancel := context.WithCancel(ctx)
	ch := make(chan AgentStreamChunk, 64)

	go c.runAgentStreamLoop(loopCtx, cancel, ch, agentStreamParams{
		tooledModel: tooledModel,
		modelID:     m.modelID,
		profile:     req.ModelProfile,
		toolMap:     toolMap,
		msgs:        msgs,
		opts:        opts,
		maxSteps:    req.MaxSteps,
		maxTokens:   req.MaxTotalTokens,
		meta:        req.Metadata,
	})

	return &agentStreamReader{ctx: loopCtx, cancel: cancel, ch: ch}, nil
}

type agentStreamParams struct {
	tooledModel model.ToolCallingChatModel
	modelID     string
	profile     ModelProfile
	toolMap     map[string]Tool
	msgs        []*schema.Message
	opts        []model.Option
	maxSteps    int
	maxTokens   int
	meta        Metadata
}

// runAgentStreamLoop is the goroutine that drives the model<->tool loop and
// sends chunks to the channel. It closes the channel on exit and cancels
// the loop context.
func (c *client) runAgentStreamLoop(ctx context.Context, cancel context.CancelFunc, ch chan<- AgentStreamChunk, p agentStreamParams) {
	defer close(ch)
	defer cancel()

	start := time.Now()
	var totalUsage Usage

	for step := 1; step <= p.maxSteps; step++ {
		// Open a streaming call with tools bound.
		einoStream, err := p.tooledModel.Stream(ctx, p.msgs, p.opts...)
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
				contentBuf.WriteString(msg.Content)
				// Forward content delta to caller in real time.
				// NOTE: this forwards content from ALL steps, including
				// intermediate ones. fullResponse (used for the Complete
				// event) is reset below for intermediate steps so only
				// the final answer is persisted.
				select {
				case ch <- AgentStreamChunk{Delta: msg.Content}:
				case <-ctx.Done():
					einoStream.Close()
					return
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
		}
		einoStream.Close()

		totalUsage.PromptTokens += stepUsage.PromptTokens
		totalUsage.CompletionTokens += stepUsage.CompletionTokens
		totalUsage.TotalTokens += stepUsage.TotalTokens

		// Enforce cumulative token budget.
		if p.maxTokens > 0 && totalUsage.TotalTokens > p.maxTokens {
			c.finishAgentStreamError(ctx, ch, p, start, totalUsage,
				fmt.Errorf("ai.StreamAgent: max total tokens exceeded (%d > %d): %w", totalUsage.TotalTokens, p.maxTokens, ErrMaxSteps))
			return
		}

		// Build the assistant message for the conversation history.
		assistantMsg := &schema.Message{
			Role:    schema.Assistant,
			Content: contentBuf.String(),
		}
		if len(toolCalls) > 0 {
			assistantMsg.ToolCalls = toolCalls
		}
		p.msgs = append(p.msgs, assistantMsg)

		// No tool calls → this is the final answer. Emit Complete with
		// this step's content. Do NOT check finishReason — if the model
		// emitted tool calls, we must execute them regardless of
		// finish_reason.
		if len(toolCalls) == 0 {
			c.finishAgentStreamOK(ctx, ch, p, start, totalUsage, contentBuf.String())
			return
		}

		// Tool calls present → this is an intermediate step. Content
		// deltas from this step (if any) were already forwarded to the
		// caller in real time; they are not included in the Complete
		// event's FullResponse (only the final step's content is).
		// Execute each tool call, emit ToolCall events,
		// and append tool results to the conversation.
		for _, tc := range toolCalls {
			tool, ok := p.toolMap[tc.Function.Name]
			if !ok {
				logx.WithContext(ctx).Errorf("ai.StreamAgent: unknown tool %q called", tc.Function.Name)
				errResult := fmt.Sprintf(`{"error":"unknown tool %q"}`, tc.Function.Name)
				select {
				case ch <- AgentStreamChunk{ToolCall: &ToolCallEvent{
					Step:  step,
					Name:  tc.Function.Name,
					Args:  tc.Function.Arguments,
					Error: errResult,
				}}:
				case <-ctx.Done():
					return
				}
				p.msgs = append(p.msgs, toEinoMessage(toolResultMessage(tc.ID, errResult)))
				continue
			}

			output, execErr := tool.Execute(ctx, tc.Function.Arguments)
			errMsg := ""
			if execErr != nil {
				logx.WithContext(ctx).Errorf("ai.StreamAgent: tool %q execution error: %v", tc.Function.Name, execErr)
				output = fmt.Sprintf(`{"error":%q}`, execErr.Error())
				errMsg = execErr.Error()
			}

			select {
			case ch <- AgentStreamChunk{ToolCall: &ToolCallEvent{
				Step:   step,
				Name:   tc.Function.Name,
				Args:   tc.Function.Arguments,
				Result: output,
				Error:  errMsg,
			}}:
			case <-ctx.Done():
				return
			}

			p.msgs = append(p.msgs, toEinoMessage(toolResultMessage(tc.ID, output)))
		}
	}

	// Hit max steps without a final answer.
	logx.WithContext(ctx).Infof("ai.StreamAgent: hit max steps %d", p.maxSteps)
	c.finishAgentStreamError(ctx, ch, p, start, totalUsage, fmt.Errorf("ai.StreamAgent: %w (max %d steps)", ErrMaxSteps, p.maxSteps))
}

// finishAgentStreamOK sends the Complete event and records metrics/usage.
func (c *client) finishAgentStreamOK(ctx context.Context, ch chan<- AgentStreamChunk, p agentStreamParams, start time.Time, totalUsage Usage, fullResponse string) {
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
