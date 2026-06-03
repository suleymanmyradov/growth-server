package ai

import (
	"encoding/json"
)

// Role enumerates message roles.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall represents a tool call requested by the model.
type ToolCall struct {
	ID   string       `json:"id"`
	Type string       `json:"type,omitempty"`
	Fn   FunctionCall `json:"function"`
}

// FunctionCall is the function name and arguments within a ToolCall.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolCallDelta represents a partial tool call during streaming.
type ToolCallDelta struct {
	Index  int    `json:"index"`
	ID     string `json:"id,omitempty"`
	FnName string `json:"function_name,omitempty"`
	FnArgs string `json:"function_arguments,omitempty"`
}

// Message is the standard role/content shape. It mirrors Eino's schema.Message
// but keeps our own type so the framework is swappable.
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content"`
	Name       string     `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// Metadata carries call-level context for logging and spend tracking.
type Metadata struct {
	UserID         string `json:"user_id,omitempty"`
	Feature        string `json:"feature,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	// PromptHash is a stable hash of the prompt template / version used for
	// this call. Enables tracing which prompt version generated a response.
	PromptHash string `json:"prompt_hash,omitempty"`
}

// ResponseFormat controls structured output.
type ResponseFormat string

const (
	ResponseFormatText       ResponseFormat = "text"
	ResponseFormatJSON       ResponseFormat = "json_object"
	ResponseFormatJSONSchema ResponseFormat = "json_schema"
)

// GenerateRequest is the input for Generate, Stream, and GenerateStructured.
type GenerateRequest struct {
	// ModelProfile selects which model to use.
	ModelProfile ModelProfile `json:"model_profile"`
	// Messages is the conversation history.
	Messages []Message `json:"messages"`
	// System is an optional system prompt prepended to Messages.
	System string `json:"system,omitempty"`
	// Temperature controls randomness.
	Temperature *float32 `json:"temperature,omitempty"`
	// MaxTokens limits output length.
	MaxTokens *int `json:"max_tokens,omitempty"`
	// ResponseFormat requests structured output from the model.
	ResponseFormat ResponseFormat `json:"response_format,omitempty"`
	// Tools are optional tool definitions for agent-style calls.
	Tools []Tool `json:"tools,omitempty"`
	// Metadata carries call-level context for logging/spend tracking.
	Metadata Metadata `json:"metadata,omitempty"`
}

// GenerateResponse is returned by Generate.
type GenerateResponse struct {
	Message   Message `json:"message"`
	Usage     Usage   `json:"usage"`
	ModelID   string  `json:"model_id"`
	LatencyMS int64   `json:"latency_ms"`
	CostUSD   float64 `json:"cost_usd"`
}

// Usage reports token counts.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Chunk is a streaming delta.
type Chunk struct {
	Delta         string         `json:"delta,omitempty"`
	ToolCallDelta *ToolCallDelta `json:"tool_call_delta,omitempty"`
	FinishReason  string         `json:"finish_reason,omitempty"`
	Usage         *Usage         `json:"usage,omitempty"`
}

// StreamReader exposes Recv/Close for streaming consumption.
type StreamReader interface {
	// Recv blocks until the next chunk is available. Returns io.EOF when
	// the stream is finished.
	Recv() (Chunk, error)
	// Close releases resources. Must be called when done reading.
	Close()
}

// AgentRequest is the input for RunAgent.
type AgentRequest struct {
	// ModelProfile selects which model to use.
	ModelProfile ModelProfile `json:"model_profile"`
	// Messages is the conversation history.
	Messages []Message `json:"messages"`
	// System is the system prompt.
	System string `json:"system,omitempty"`
	// Tools the agent may call.
	Tools []Tool `json:"tools"`
	// MaxSteps limits the model<->tool round-trip loop.
	MaxSteps int `json:"max_steps"`
	// MaxTokens limits output length per generation step.
	MaxTokens *int `json:"max_tokens,omitempty"`
	// MaxTotalTokens limits the cumulative prompt + completion tokens across
	// all agent steps. Zero means no limit. Exceeding this aborts the loop.
	MaxTotalTokens int `json:"max_total_tokens,omitempty"`
	// Temperature controls randomness.
	Temperature *float32 `json:"temperature,omitempty"`
	// Metadata carries call-level context for logging/spend tracking.
	Metadata Metadata `json:"metadata,omitempty"`
}

// AgentResponse is returned by RunAgent.
type AgentResponse struct {
	Messages  []Message `json:"messages"`
	Usage     Usage     `json:"usage"`
	ModelID   string    `json:"model_id"`
	Steps     int       `json:"steps"`
	LatencyMS int64     `json:"latency_ms"`
	CostUSD   float64   `json:"cost_usd"`
}

// toEinoMessages converts our Messages to Eino schema.Message pointers.
func toEinoMessages(msgs []Message, system string) []*einoMessage {
	var out []*einoMessage
	if system != "" {
		out = append(out, &einoMessage{
			Role:    einoSystem,
			Content: system,
		})
	}
	for _, m := range msgs {
		out = append(out, toEinoMessage(m))
	}
	return out
}

// toEinoMessage converts a single Message to an Eino schema.Message.
func toEinoMessage(m Message) *einoMessage {
	em := &einoMessage{
		Role:    toEinoRole(m.Role),
		Content: m.Content,
		Name:    m.Name,
	}
	if m.ToolCallID != "" {
		em.ToolCallID = m.ToolCallID
	}
	if len(m.ToolCalls) > 0 {
		em.ToolCalls = make([]einoToolCall, len(m.ToolCalls))
		for i, tc := range m.ToolCalls {
			em.ToolCalls[i] = einoToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: einoFunctionCall{
					Name:      tc.Fn.Name,
					Arguments: tc.Fn.Arguments,
				},
			}
		}
	}
	return em
}

// fromEinoMessage converts an Eino schema.Message to our Message.
func fromEinoMessage(em *einoMessage) Message {
	m := Message{
		Role:    fromEinoRole(em.Role),
		Content: em.Content,
		Name:    em.Name,
	}
	if em.ToolCallID != "" {
		m.ToolCallID = em.ToolCallID
	}
	if len(em.ToolCalls) > 0 {
		m.ToolCalls = make([]ToolCall, len(em.ToolCalls))
		for i, tc := range em.ToolCalls {
			m.ToolCalls[i] = ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Fn: FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}
	return m
}

// toolResultMessage creates a tool result Message from a tool execution.
func toolResultMessage(toolCallID, content string) Message {
	return Message{
		Role:       RoleTool,
		Content:    content,
		ToolCallID: toolCallID,
	}
}

// extractToolResults extracts tool call results from JSON-encoded content.
func extractToolCallArgs(argsJSON string) map[string]any {
	var m map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &m); err != nil {
		return nil
	}
	return m
}
