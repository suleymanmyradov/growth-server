package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

// RunAgent runs the model<->tool round-trip loop until the model returns
// a final message (no tool calls) or maxSteps is hit.
func (c *client) RunAgent(ctx context.Context, req AgentRequest) (AgentResponse, error) {
	if len(req.Tools) == 0 {
		return AgentResponse{}, ErrNoTools
	}
	if req.MaxSteps <= 0 {
		req.MaxSteps = 10
	}

	if err := c.checkQuota(ctx, req.Metadata); err != nil {
		return AgentResponse{}, err
	}

	m, err := c.modelFor(req.ModelProfile)
	if err != nil {
		return AgentResponse{}, err
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

	start := time.Now()
	var totalUsage Usage
	var allMessages []Message
	steps := 0

	for step := 0; step < req.MaxSteps; step++ {
		steps++

		result, err := c.callGenerateWithTools(ctx, m, msgs, toolInfos, opts)
		if err != nil {
			c.logCall(ctx, req.ModelProfile, m.modelID, req.Metadata, totalUsage, time.Since(start).Milliseconds(), 0, err)
			recordMetrics(req.ModelProfile, m.modelID, "error")
			return AgentResponse{}, fmt.Errorf("ai.RunAgent step %d: %w", step+1, err)
		}

		ourMsg := fromEinoMessage(result)
		allMessages = append(allMessages, ourMsg)
		msgs = append(msgs, result)

		// Accumulate usage.
		if result.ResponseMeta != nil && result.ResponseMeta.Usage != nil {
			u := result.ResponseMeta.Usage
			totalUsage.PromptTokens += u.PromptTokens
			totalUsage.CompletionTokens += u.CompletionTokens
			totalUsage.TotalTokens += u.TotalTokens
		}

		// If no tool calls, the agent is done.
		if len(result.ToolCalls) == 0 {
			break
		}

		// Execute each tool call and append results.
		for _, tc := range result.ToolCalls {
			tool, ok := toolMap[tc.Function.Name]
			if !ok {
				logx.WithContext(ctx).Errorf("ai.RunAgent: unknown tool %q called", tc.Function.Name)
				toolResult := toolResultMessage(tc.ID, fmt.Sprintf(`{"error":"unknown tool %q"}`, tc.Function.Name))
				allMessages = append(allMessages, toolResult)
				msgs = append(msgs, toEinoMessage(toolResult))
				continue
			}

			output, err := tool.Execute(ctx, tc.Function.Arguments)
			if err != nil {
				logx.WithContext(ctx).Errorf("ai.RunAgent: tool %q execution error: %v", tc.Function.Name, err)
				output = fmt.Sprintf(`{"error":%q}`, err.Error())
			}

			toolResult := toolResultMessage(tc.ID, output)
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
	recordMetrics(req.ModelProfile, m.modelID, "ok")

	return AgentResponse{
		Messages:  allMessages,
		Usage:     totalUsage,
		ModelID:   m.modelID,
		Steps:     steps,
		LatencyMS: latencyMS,
		CostUSD:   costUSD,
	}, nil
}
