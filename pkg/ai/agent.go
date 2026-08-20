package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/zeromicro/go-zero/core/logx"
)

// RunAgent runs the model<->tool round-trip loop until the model returns
// a final message (no tool calls) or maxSteps is hit.
func (c *client) RunAgent(ctx context.Context, req AgentRequest) (AgentResponse, error) {
	prep, err := c.prepareAgent(ctx, &req)
	if err != nil {
		return AgentResponse{}, err
	}

	m := prep.m
	toolInfos := prep.toolInfos
	toolMap := prep.toolMap
	msgs := prep.msgs
	opts := prep.opts

	start := time.Now()
	var totalUsage Usage
	var allMessages []Message
	steps := 0

	// Captured thought_signatures from the previous step's tool calls.
	// Gemini requires these on assistant tool_calls in the conversation
	// history; the transport injects them into the request.
	var prevSignatures map[string]string

	for step := 0; step < req.MaxSteps; step++ {
		steps++

		// Per-step context: carry a fresh capture for the response, and
		// the previous step's signatures for request injection.
		capture := newThoughtSignatureCapture()
		stepCtx := withCapture(ctx, capture)
		if len(prevSignatures) > 0 {
			stepCtx = withInject(stepCtx, prevSignatures)
		}

		result, err := c.callGenerateWithTools(stepCtx, m, msgs, toolInfos, opts)
		if err != nil {
			// Try the fallback chain before giving up on this turn. On
			// success, rebind m to the fallback model so subsequent steps
			// and final usage/metrics attribute to it. Mirrors tryFallback
			// for the non-agent Generate path.
			fb, fbResult, fbErr := c.runAgentFallback(stepCtx, req.ModelProfile, m, msgs, toolInfos, opts, err)
			if fbErr == nil {
				m = fb
				result = fbResult
			} else {
				latencyMS := time.Since(start).Milliseconds()
				c.logCall(ctx, req.ModelProfile, m.modelID, req.Metadata, totalUsage, latencyMS, 0, err)
				recordMetrics(req.ModelProfile, m.modelID, "error", req.Metadata.Feature, totalUsage, 0, latencyMS)
				return AgentResponse{}, fmt.Errorf("ai.RunAgent step %d: %w", step+1, err)
			}
		}

		ourMsg := fromEinoMessage(result)
		allMessages = append(allMessages, ourMsg)
		msgs = append(msgs, result)

		// Accumulate usage.
		if result.ResponseMeta != nil && result.ResponseMeta.Usage != nil {
			accumulateUsage(&totalUsage, Usage{
				PromptTokens:     result.ResponseMeta.Usage.PromptTokens,
				CompletionTokens: result.ResponseMeta.Usage.CompletionTokens,
				TotalTokens:      result.ResponseMeta.Usage.TotalTokens,
			})
		}

		// Enforce cumulative token budget.
		if req.MaxTotalTokens > 0 && totalUsage.TotalTokens > req.MaxTotalTokens {
			latencyMS := time.Since(start).Milliseconds()
			c.logCall(ctx, req.ModelProfile, m.modelID, req.Metadata, totalUsage, latencyMS, 0, ErrMaxTokens)
			recordMetrics(req.ModelProfile, m.modelID, "error", req.Metadata.Feature, totalUsage, 0, latencyMS)
			return AgentResponse{}, fmt.Errorf("ai.RunAgent: max total tokens exceeded (%d > %d): %w", totalUsage.TotalTokens, req.MaxTotalTokens, ErrMaxTokens)
		}

		// If no tool calls, the agent is done.
		if len(result.ToolCalls) == 0 {
			break
		}

		// Preserve captured thought_signatures for the next step's
		// request injection (Gemini 3.x requires them on tool_calls).
		// Merge across steps: the conversation history accumulates tool
		// calls from ALL steps, and each needs its signature on the
		// next request. Replacing would drop earlier steps' signatures
		// and cause 400 "missing thought_signature" errors.
		if prevSignatures == nil {
			prevSignatures = make(map[string]string)
		}
		for k, v := range capture.all() {
			prevSignatures[k] = v
		}

		// Execute each tool call and append results.
		for _, tc := range result.ToolCalls {
			execRes := executeToolCall(ctx, toolMap, tc.Function.Name, tc.Function.Arguments, "ai.RunAgent")
			toolResult := toolResultMessage(tc.ID, execRes.Output)
			allMessages = append(allMessages, toolResult)
			msgs = append(msgs, toEinoMessage(toolResult))
		}
	}

	// Check if we hit max steps.
	if steps >= req.MaxSteps && len(msgs) > 0 {
		lastMsg := msgs[len(msgs)-1]
		if len(lastMsg.ToolCalls) > 0 {
			logx.WithContext(ctx).Infof("ai.RunAgent: hit max steps %d", req.MaxSteps)
		}
	}

	costUSD := c.cfg.ComputeCost(m.modelID, totalUsage.PromptTokens, totalUsage.CompletionTokens)
	latencyMS := time.Since(start).Milliseconds()

	c.recordUsage(ctx, req.Metadata, totalUsage, costUSD)
	c.logCall(ctx, req.ModelProfile, m.modelID, req.Metadata, totalUsage, latencyMS, costUSD, nil)
	recordMetrics(req.ModelProfile, m.modelID, "ok", req.Metadata.Feature, totalUsage, costUSD, latencyMS)

	return AgentResponse{
		Messages:  allMessages,
		Usage:     totalUsage,
		ModelID:   m.modelID,
		Steps:     steps,
		LatencyMS: latencyMS,
		CostUSD:   costUSD,
	}, nil
}

// runAgentFallback tries the fallback chain for RunAgent after the primary
// model fails to generate at a given step. The chain is resolved from the
// originally requested profile, and the primary model is skipped. On success
// it returns the fallback model that served the request (so the caller can
// rebind m for subsequent steps) and the result message. On total failure it
// returns the last error.
//
// This mirrors tryFallback for the non-agent Generate path. Without it,
// RunAgent has no fallback and a single 5xx/429 from the primary model kills
// the whole agentic turn even when a fallback chain is configured.
func (c *client) runAgentFallback(ctx context.Context, origProfile ModelProfile, primary openaiModel, msgs []*schema.Message, toolInfos []*schema.ToolInfo, opts []model.Option, primaryErr error) (openaiModel, *schema.Message, error) {
	if !c.cfg.FallbackPolicy.Enabled {
		return openaiModel{}, nil, primaryErr
	}
	chain := c.fallbackModelsFor(origProfile)
	if len(chain) == 0 {
		return openaiModel{}, nil, primaryErr
	}

	logx.WithContext(ctx).Infof("ai.RunAgent: primary model %s failed, trying %d fallback(s): %v", primary.modelID, len(chain), primaryErr)

	lastErr := primaryErr
	for i, fb := range chain {
		if fb.modelID == primary.modelID {
			continue // never retry the model that just failed
		}
		logx.WithContext(ctx).Infof("ai.RunAgent: trying fallback %d/%d: %s", i+1, len(chain), fb.modelID)

		result, err := c.callGenerateWithTools(ctx, fb, msgs, toolInfos, opts)
		if err != nil {
			lastErr = err
			logx.WithContext(ctx).Infof("ai.RunAgent: fallback %d/%d (%s) also failed: %v", i+1, len(chain), fb.modelID, err)
			continue
		}

		logx.WithContext(ctx).Infof("ai.RunAgent: fallback %d/%d (%s) succeeded", i+1, len(chain), fb.modelID)
		return fb, result, nil
	}

	return openaiModel{}, nil, fmt.Errorf("ai.RunAgent: all %d fallback(s) failed: %w (primary: %v)", len(chain), lastErr, primaryErr)
}
