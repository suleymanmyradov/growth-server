package ai

import (
	"encoding/json"

	"github.com/cloudwego/eino/schema"
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

// Attachment is a user-supplied file that should be passed to the model as
// a multimodal content part. Data is base64-encoded.
type Attachment struct {
	Type        string `json:"type"` // "image" or "document"
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
	Data        string `json:"data"` // base64-encoded bytes
}

// Message is the standard role/content shape. It mirrors Eino's schema.Message
// but keeps our own type so the framework is swappable.
type Message struct {
	Role        Role         `json:"role"`
	Content     string       `json:"content"`
	Reasoning   string       `json:"reasoning,omitempty"` // thinking content from <thought> tags (Gemma-4)
	Name        string       `json:"name,omitempty"`
	ToolCalls   []ToolCall   `json:"tool_calls,omitempty"`
	ToolCallID  string       `json:"tool_call_id,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Metadata carries call-level context for logging and spend tracking.
type Metadata struct {
	UserID  string `json:"user_id,omitempty"`
	Feature string `json:"feature,omitempty"`

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
	// DisableReasoning sends reasoning:{enabled:false} to OpenRouter,
	// disabling the thinking/reasoning phase for reasoning models (e.g. nvidia nemotron).
	DisableReasoning bool `json:"disable_reasoning,omitempty"`
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
	Reasoning     string         `json:"reasoning,omitempty"` // thinking delta from <thought> tags (Gemma-4)
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

// AgentStreamChunk is one event from an agent stream (StreamAgent).
//
// A chunk is exactly one of:
//   - Reasoning: a reasoning/thinking content delta streamed in real time.
//     Only reasoning models (e.g. DeepSeek R1, GPT-5 with reasoning) emit
//     this; for non-reasoning models it is never set. Surfaced to the user
//     as the model's live "thinking" process.
//   - Delta: a text content delta streamed in real time. Deltas from
//     intermediate steps (model reasoning before a tool call) and the
//     final answer are both forwarded. Only the final step's content
//     is included in the Complete event's FullResponse.
//   - ToolCall: a tool was called and executed during an intermediate step.
//     The caller can surface this as a status update ("Looking up your
//     habits...").
//   - Complete: the agent loop is done. FullResponse carries the entire
//     final answer; Usage carries cumulative token usage.
//   - Error: an error occurred mid-stream. The stream ends after this.
type AgentStreamChunk struct {
	Reasoning    string         `json:"reasoning,omitempty"`
	Delta        string         `json:"delta,omitempty"`
	ToolCall     *ToolCallEvent `json:"tool_call,omitempty"`
	Complete     bool           `json:"complete,omitempty"`
	FullResponse string         `json:"full_response,omitempty"`
	// FinishReason carries the provider's finish_reason for the final step
	// (e.g. "stop", "length", "tool_calls"). On the Complete chunk, a value
	// of "length" signals that the provider truncated the output at its
	// max_tokens limit — the caller should log a warning or retry, since
	// FullResponse is incomplete. Empty when the provider doesn't report
	// a finish_reason.
	FinishReason string `json:"finish_reason,omitempty"`
	Usage        *Usage `json:"usage,omitempty"`
	Error        error  `json:"-"`
}

// ToolCallEvent describes a tool invocation during the agent loop.
// Two events are emitted per tool call:
//   - Status "started": emitted before tool.Execute, carries Name and Args
//     only. Lets the caller show a "Looking up..." status immediately.
//   - Status "completed": emitted after tool.Execute returns, carries
//     Result and Error. The caller can render proposals or error states.
type ToolCallEvent struct {
	Step   int    `json:"step"`             // 1-based step in the loop
	Name   string `json:"name"`             // tool name
	Status string `json:"status"`           // "started" or "completed"
	Args   string `json:"args,omitempty"`   // JSON arguments
	Result string `json:"result,omitempty"` // JSON result (set on "completed")
	Error  string `json:"error,omitempty"`  // non-empty if execution failed (set on "completed")
}

// Tool status values for ToolCallEvent.Status.
const (
	ToolStatusStarted   = "started"
	ToolStatusCompleted = "completed"
)

// AgentStreamReader exposes Recv/Close for streaming agent consumption.
type AgentStreamReader interface {
	// Recv blocks until the next chunk is available. Returns io.EOF when
	// the stream is finished.
	Recv() (AgentStreamChunk, error)
	// Close releases resources. Must be called when done reading.
	Close()
}

// toEinoMessages converts our Messages to Eino schema.Message pointers.
func toEinoMessages(msgs []Message, system string) []*einoMessage {
	var out []*einoMessage
	if system != "" {
		out = make([]*einoMessage, 0, len(msgs)+1)
		out = append(out, &einoMessage{
			Role:    einoSystem,
			Content: system,
		})
	} else {
		out = make([]*einoMessage, 0, len(msgs))
	}
	for _, m := range msgs {
		out = append(out, toEinoMessage(m))
	}
	return out
}

// toEinoMessage converts a single Message to an Eino schema.Message.
// When the message has attachments, the text becomes the first part of a
// multimodal user_input_multi_content array (Eino's native multimodal shape).
func toEinoMessage(m Message) *einoMessage {
	em := &einoMessage{
		Role: toEinoRole(m.Role),
		Name: m.Name,
	}
	if len(m.Attachments) > 0 {
		parts := make([]schema.MessageInputPart, 0, len(m.Attachments)+1)
		if m.Content != "" {
			parts = append(parts, schema.MessageInputPart{
				Type: schema.ChatMessagePartTypeText,
				Text: m.Content,
			})
		}
		for _, a := range m.Attachments {
			parts = append(parts, toEinoAttachmentPart(a))
		}
		em.UserInputMultiContent = parts
	} else {
		em.Content = m.Content
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

// toEinoAttachmentPart converts an ai.Attachment to an Eino MessageInputPart.
// Images are emitted as image_url parts, all other files as file_url parts.
func toEinoAttachmentPart(a Attachment) schema.MessageInputPart {
	switch a.Type {
	case "image":
		return schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{
					Base64Data: &a.Data,
					MIMEType:   a.ContentType,
				},
			},
		}
	}
	return schema.MessageInputPart{
		Type: schema.ChatMessagePartTypeFileURL,
		File: &schema.MessageInputFile{
			MessagePartCommon: schema.MessagePartCommon{
				Base64Data: &a.Data,
				MIMEType:   a.ContentType,
			},
			Name: a.Name,
		},
	}
}

// fromEinoMessage converts an Eino schema.Message to our Message.
// Splits <thought>...</thought> tags that Gemma-4 models emit inline in the
// content field into the Reasoning field (thinking mode cannot be disabled
// for these models). The frontend's existing reasoning UI displays it.
func fromEinoMessage(em *einoMessage) Message {
	parts := splitThoughtTags(em.Content)
	m := Message{
		Role:      fromEinoRole(em.Role),
		Content:   parts.Content,
		Reasoning: parts.Reasoning,
		Name:      em.Name,
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
