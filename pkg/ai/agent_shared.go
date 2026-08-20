package ai

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/zeromicro/go-zero/core/logx"
)

// agentPrepared holds the common state prepared from an AgentRequest,
// shared by RunAgent and StreamAgent to avoid duplicated initialization.
type agentPrepared struct {
	m         openaiModel
	toolInfos []*schema.ToolInfo
	toolMap   map[string]Tool
	msgs      []*schema.Message
	opts      []model.Option
}

// prepareAgent performs the shared initialization that both RunAgent and
// StreamAgent need before starting the agent loop:
//   - validate tools (at least one required)
//   - default MaxSteps to 10 when zero
//   - check quota
//   - resolve the model for the requested profile
//   - build tool infos and the tool-name map
//   - convert messages to Eino format
//   - build model options (temperature, max_tokens)
//
// The caller is responsible for binding tools (WithTools) since the
// streaming and non-streaming paths bind at different points.
func (c *client) prepareAgent(ctx context.Context, req *AgentRequest) (*agentPrepared, error) {
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

	return &agentPrepared{
		m:         m,
		toolInfos: toolInfos,
		toolMap:   toolMap,
		msgs:      msgs,
		opts:      opts,
	}, nil
}

// toolExecResult holds the result of executing a single tool call.
type toolExecResult struct {
	Output string // JSON result (or error JSON if execution failed)
	ErrMsg string // non-empty if execution failed; empty on success
}

// executeToolCall looks up a tool by name, executes it with the given
// arguments, and returns the result. If the tool is unknown or execution
// fails, the result contains an error JSON string and a non-empty ErrMsg.
// The returned Output is always safe to append to the conversation as a
// tool result message.
//
// Shared by RunAgent and StreamAgent to avoid duplicated tool-execution
// and error-handling logic.
func executeToolCall(ctx context.Context, toolMap map[string]Tool, name, args string, logPrefix string) toolExecResult {
	tool, ok := toolMap[name]
	if !ok {
		logx.WithContext(ctx).Errorf("%s: unknown tool %q called", logPrefix, name)
		errResult := fmt.Sprintf(`{"error":"unknown tool %q"}`, name)
		return toolExecResult{
			Output: errResult,
			ErrMsg: errResult,
		}
	}

	output, err := tool.Execute(ctx, args)
	if err != nil {
		logx.WithContext(ctx).Errorf("%s: tool %q execution error: %v", logPrefix, name, err)
		return toolExecResult{
			Output: fmt.Sprintf(`{"error":%q}`, err.Error()),
			ErrMsg: err.Error(),
		}
	}

	return toolExecResult{Output: output}
}

// accumulateUsage adds step usage into the cumulative total.
// Shared by RunAgent and StreamAgent.
func accumulateUsage(total *Usage, step Usage) {
	total.PromptTokens += step.PromptTokens
	total.CompletionTokens += step.CompletionTokens
	total.TotalTokens += step.TotalTokens
}
