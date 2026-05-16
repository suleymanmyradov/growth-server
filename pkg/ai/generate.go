package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/zeromicro/go-zero/core/logx"
)

// Generate performs one-shot generation.
func (c *client) Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error) {
	if err := c.checkQuota(ctx, req.Metadata); err != nil {
		return GenerateResponse{}, err
	}

	m, err := c.modelFor(req.ModelProfile)
	if err != nil {
		return GenerateResponse{}, err
	}

	msgs := toEinoMessages(req.Messages, req.System)
	opts := einoModelOptions(req)

	// If tools are provided, use tool-calling model.
	if len(req.Tools) > 0 {
		toolInfos := buildToolInfos(req.Tools)
		return c.generateWithTools(ctx, m, req, msgs, toolInfos, opts)
	}

	start := time.Now()
	result, err := c.callGenerate(ctx, m, msgs, opts)
	latencyMS := time.Since(start).Milliseconds()

	if err != nil {
		// Try fallback if configured.
		if resp, fbErr := c.tryFallback(ctx, req, msgs, opts, err, latencyMS); fbErr == nil {
			return resp, nil
		}
		c.logCall(ctx, req.ModelProfile, m.modelID, req.Metadata, Usage{}, latencyMS, 0, err)
		recordMetrics(req.ModelProfile, m.modelID, "error")
		return GenerateResponse{}, fmt.Errorf("ai.Generate: %w", err)
	}

	usage := extractUsage(result)
	costUSD := c.cfg.ComputeCost(m.modelID, usage.PromptTokens, usage.CompletionTokens)
	latencyMS = time.Since(start).Milliseconds()

	c.recordUsage(ctx, req.Metadata, usage, costUSD)
	c.logCall(ctx, req.ModelProfile, m.modelID, req.Metadata, usage, latencyMS, costUSD, nil)
	recordMetrics(req.ModelProfile, m.modelID, "ok")

	return GenerateResponse{
		Message:   fromEinoMessage(result),
		Usage:     usage,
		ModelID:   m.modelID,
		LatencyMS: latencyMS,
		CostUSD:   costUSD,
	}, nil
}

// generateWithTools handles generation when tools are provided (but not in agent loop).
func (c *client) generateWithTools(ctx context.Context, m openaiModel, req GenerateRequest, msgs []*schema.Message, toolInfos []*schema.ToolInfo, opts []model.Option) (GenerateResponse, error) {
	start := time.Now()
	result, err := c.callGenerateWithTools(ctx, m, msgs, toolInfos, opts)
	latencyMS := time.Since(start).Milliseconds()

	if err != nil {
		c.logCall(ctx, req.ModelProfile, m.modelID, req.Metadata, Usage{}, latencyMS, 0, err)
		recordMetrics(req.ModelProfile, m.modelID, "error")
		return GenerateResponse{}, fmt.Errorf("ai.Generate: %w", err)
	}

	usage := extractUsage(result)
	costUSD := c.cfg.ComputeCost(m.modelID, usage.PromptTokens, usage.CompletionTokens)
	latencyMS = time.Since(start).Milliseconds()

	c.recordUsage(ctx, req.Metadata, usage, costUSD)
	c.logCall(ctx, req.ModelProfile, m.modelID, req.Metadata, usage, latencyMS, costUSD, nil)
	recordMetrics(req.ModelProfile, m.modelID, "ok")

	return GenerateResponse{
		Message:   fromEinoMessage(result),
		Usage:     usage,
		ModelID:   m.modelID,
		LatencyMS: latencyMS,
		CostUSD:   costUSD,
	}, nil
}

// tryFallback attempts the fallback model if the primary model fails.
func (c *client) tryFallback(ctx context.Context, req GenerateRequest, msgs []*schema.Message, opts []model.Option, primaryErr error, primaryLatencyMS int64) (GenerateResponse, error) {
	if !c.cfg.FallbackPolicy.Enabled {
		return GenerateResponse{}, primaryErr
	}
	fb, ok := c.fallbackModel()
	if !ok {
		return GenerateResponse{}, primaryErr
	}

	logx.WithContext(ctx).Infof("ai: primary model failed, trying fallback: %v", primaryErr)

	start := time.Now()
	result, err := c.callGenerate(ctx, fb, msgs, opts)
	latencyMS := time.Since(start).Milliseconds()
	if err != nil {
		c.logCall(ctx, ModelFallback, fb.modelID, req.Metadata, Usage{}, latencyMS, 0, err)
		recordMetrics(ModelFallback, fb.modelID, "error")
		return GenerateResponse{}, fmt.Errorf("ai.Generate fallback also failed: %w (primary: %v)", err, primaryErr)
	}

	usage := extractUsage(result)
	costUSD := c.cfg.ComputeCost(fb.modelID, usage.PromptTokens, usage.CompletionTokens)
	c.recordUsage(ctx, req.Metadata, usage, costUSD)
	c.logCall(ctx, ModelFallback, fb.modelID, req.Metadata, usage, latencyMS, costUSD, nil)
	recordMetrics(ModelFallback, fb.modelID, "ok")

	return GenerateResponse{
		Message:   fromEinoMessage(result),
		Usage:     usage,
		ModelID:   fb.modelID,
		LatencyMS: primaryLatencyMS + latencyMS,
		CostUSD:   costUSD,
	}, nil
}

// GenerateStructured wraps Generate + JSON schema validation.
func (c *client) GenerateStructured(ctx context.Context, req GenerateRequest, out any) error {
	// Force JSON output.
	req.ResponseFormat = ResponseFormatJSON

	resp, err := c.Generate(ctx, req)
	if err != nil {
		return fmt.Errorf("ai.GenerateStructured: %w", err)
	}

	content := strings.TrimSpace(resp.Message.Content)
	// Strip markdown code fences if present.
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	if err := json.Unmarshal([]byte(content), out); err != nil {
		return fmt.Errorf("ai.GenerateStructured: unmarshal: %w (content: %q)", err, truncate(content, 200))
	}
	return nil
}

// extractUsage extracts token usage from an Eino Message.
func extractUsage(msg *schema.Message) Usage {
	if msg.ResponseMeta == nil || msg.ResponseMeta.Usage == nil {
		return Usage{}
	}
	u := msg.ResponseMeta.Usage
	return Usage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
	}
}

// truncate shortens a string for error messages.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
