package ai_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/ai/aitest"
	"github.com/suleymanmyradov/growth-server/pkg/ai/memory"
	"github.com/suleymanmyradov/growth-server/pkg/ai/safety"
)

// Example_feedback demonstrates Phase 3C short feedback generation.
func Example_feedback() {
	cfg := ai.Config{
		APIKey: "OPENROUTER_API_KEY",
	}
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}

	client, err := ai.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.Generate(context.Background(), ai.GenerateRequest{
		ModelProfile: ai.ModelCheap,
		System:       "You are a supportive accountability coach. Give brief, encouraging feedback.",
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "I completed my daily coding goal!"},
		},
		Metadata: ai.Metadata{UserID: "user123", Feature: "check-in-feedback"},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.Message.Content)
}

// Example_streamingChat demonstrates Phase 5 streaming chat with mock client.
func Example_streamingChat() {
	mc := aitest.NewMockClient()
	mc.RecordStream(ai.ModelChat,
		ai.Chunk{Delta: "Hello "},
		ai.Chunk{Delta: "there! "},
		ai.Chunk{Delta: "How can I help?"},
	)

	sr, err := mc.Stream(context.Background(), ai.GenerateRequest{
		ModelProfile: ai.ModelChat,
		Messages:     []ai.Message{{Role: ai.RoleUser, Content: "Hi coach!"}},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer sr.Close()

	for {
		chunk, err := sr.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		fmt.Print(chunk.Delta)
	}
	// Output: Hello there! How can I help?
}

// Example_agentWithTools demonstrates Phase 5 agent with tool calling using mock.
func Example_agentWithTools() {
	type echoInput struct {
		Text string `json:"text" jsonschema:"required,description=Text to echo"`
	}
	type echoOutput struct {
		Echo string `json:"echo"`
	}

	echoTool := ai.NewTool[echoInput, echoOutput](ai.ToolSpec{
		Name:        "echo",
		Description: "Echoes back the input text.",
		Handler: func(_ context.Context, in echoInput) (echoOutput, error) {
			return echoOutput{Echo: in.Text}, nil
		},
	})

	mc := aitest.NewMockClient()
	mc.RecordResponse(ai.ModelChat, ai.Message{
		Role:    ai.RoleAssistant,
		Content: "The echo tool returned: hello",
	}, ai.Usage{PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30}, 0.001)

	resp, err := mc.RunAgent(context.Background(), ai.AgentRequest{
		ModelProfile: ai.ModelChat,
		Messages:     []ai.Message{{Role: ai.RoleUser, Content: "Use the echo tool with 'hello'"}},
		Tools:        []ai.Tool{echoTool},
		MaxSteps:     5,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.Steps)
	// Output: 1
}

// Example_conversationWindow demonstrates memory windowing.
func Example_conversationWindow() {
	w := memory.NewConversationWindow(3)
	w.Add(
		ai.Message{Role: ai.RoleSystem, Content: "You are a coach."},
		ai.Message{Role: ai.RoleUser, Content: "Hi"},
		ai.Message{Role: ai.RoleAssistant, Content: "Hello!"},
		ai.Message{Role: ai.RoleUser, Content: "How are you?"},
	)

	for _, m := range w.Messages() {
		fmt.Printf("[%s] %s\n", m.Role, m.Content)
	}
	// Output:
	// [system] You are a coach.
	// [assistant] Hello!
	// [user] How are you?
}

// Example_safetyClassifier demonstrates the safety classifier with mock.
func Example_safetyClassifier() {
	mc := aitest.NewMockClient()
	mc.RecordResponse(ai.ModelClassifier, ai.Message{
		Role:    ai.RoleAssistant,
		Content: `{"category":"safe","confidence":0.99,"reason":"no safety concern"}`,
	}, ai.Usage{}, 0)

	classifier := safety.NewLLMClassifier(mc)
	verdict, err := classifier.Classify(context.Background(), "I had a great day!")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s %.2f\n", verdict.Category, verdict.Confidence)
	// Output: safe 0.99
}

// Example_configDefaults demonstrates config with defaults.
func Example_configDefaults() {
	cfg := ai.Config{APIKey: "sk-test"}
	_ = cfg.Validate()

	fmt.Println(cfg.BaseURL)
	fmt.Println(cfg.MaxRetries)
	fmt.Println(cfg.DefaultTimeout == 30*time.Second)
	// Output:
	// https://openrouter.ai/api/v1
	// 3
	// true
}
