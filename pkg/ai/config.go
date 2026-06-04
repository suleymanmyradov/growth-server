package ai

import (
	"fmt"
	"time"
)

// ModelProfile identifies the intent of a model call. Callers pick by intent,
// not by model string. The mapping to concrete model IDs lives in Config.Models.
type ModelProfile string

const (
	// ModelCheap is for high-volume, latency-sensitive single-shot generation
	// (check-in feedback, notification copy).
	ModelCheap ModelProfile = "cheap"

	// ModelCheapLong is for longer structured generation (weekly review).
	ModelCheapLong ModelProfile = "cheap_long"

	// ModelClassifier is for the safety classifier — cheap and fast.
	ModelClassifier ModelProfile = "classifier"

	// ModelChat is for conversational multi-turn chat with tool calling.
	ModelChat ModelProfile = "chat"

	// ModelFallback is the quality fallback when ModelChat fails.
	ModelFallback ModelProfile = "fallback"
)

// DefaultModels maps each profile to its default model ID via OpenRouter.
// Override per-environment through Config.Models.
var DefaultModels = map[ModelProfile]string{
	ModelCheap:      "deepseek/deepseek-chat-v3",
	ModelCheapLong:  "google/gemini-2.0-flash-001",
	ModelClassifier: "google/gemini-2.0-flash-lite-001",
	ModelChat:       "openai/gpt-4o-mini",
	ModelFallback:   "anthropic/claude-3.5-haiku",
}

// CostRate describes the per-1K-token cost in USD for a model.
type CostRate struct {
	PromptPer1K     float64
	CompletionPer1K float64
}

// DefaultCostRates maps model IDs to their per-1K-token pricing.
// Used to compute cost_usd for observability.
var DefaultCostRates = map[string]CostRate{
	"deepseek/deepseek-chat-v3":        {PromptPer1K: 0.0001, CompletionPer1K: 0.0002},
	"google/gemini-2.0-flash-001":      {PromptPer1K: 0.0001, CompletionPer1K: 0.0004},
	"google/gemini-2.0-flash-lite-001": {PromptPer1K: 0.0000075, CompletionPer1K: 0.00003},
	"openai/gpt-4o-mini":               {PromptPer1K: 0.00015, CompletionPer1K: 0.0006},
	"anthropic/claude-3.5-haiku":       {PromptPer1K: 0.0008, CompletionPer1K: 0.004},
}

// FallbackPolicy controls automatic retry with a fallback model.
type FallbackPolicy struct {
	// Enabled turns on fallback behaviour.
	Enabled bool
	// MaxFailures is the number of failures on the primary model before
	// retrying once with the fallback model.
	MaxFailures int
}

// Config holds all configuration for the AI client. It is loadable from
// go-zero YAML configs via conf.MustLoad.
type Config struct {
	// APIKey is the OpenRouter API key (env: OPENROUTER_API_KEY). Never logged.
	APIKey string `json:"api_key"`
	// BaseURL defaults to https://openrouter.ai/api/v1.
	BaseURL string `json:"base_url"`
	// Models maps ModelProfile to model ID. Merged over DefaultModels.
	Models map[ModelProfile]string `json:"models"`
	// CostRates maps model ID to per-1K-token pricing. Merged over DefaultCostRates.
	CostRates map[string]CostRate `json:"cost_rates"`
	// DefaultTimeout per request.
	DefaultTimeout time.Duration `json:"default_timeout"`
	// MaxRetries for transient errors (429 / 5xx).
	MaxRetries int `json:"max_retries"`
	// RetryBackoff is the initial backoff duration; doubles each retry.
	RetryBackoff time.Duration `json:"retry_backoff"`
	// HTTPReferer is the OpenRouter analytics HTTP-Referer header.
	HTTPReferer string `json:"http_referer"`
	// XTitle is the OpenRouter analytics X-Title header.
	XTitle string `json:"x_title"`
	// FallbackPolicy controls automatic retry with a fallback model.
	FallbackPolicy FallbackPolicy `json:"fallback_policy"`
	// LogPrompts enables logging of prompt/completion contents at info level.
	// Only use in development; never enable in production.
	LogPrompts bool `json:"log_prompts"`
	// Quota configuration (optional; requires Redis).
	Quota QuotaConfig `json:"quota"`
}

// QuotaConfig controls per-user and global daily spend caps.
type QuotaConfig struct {
	// RedisAddr is the Redis address for quota tracking.
	RedisAddr string `json:"redis_addr"`
	// RedisPassword for Redis auth.
	RedisPassword string `json:"redis_password"`
	// RedisDB selects the Redis database number.
	RedisDB int `json:"redis_db"`
	// UserDailyTokenCap is the max tokens a single user can consume per day.
	// 0 means unlimited.
	UserDailyTokenCap int64 `json:"user_daily_token_cap"`
	// GlobalDailyCostCapUSD is the max total spend across all users per day.
	// 0 means unlimited.
	GlobalDailyCostCapUSD float64 `json:"global_daily_cost_cap_usd"`
}

// Validate checks required fields and applies defaults.
func (c *Config) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("ai.Config: APIKey is required")
	}
	if c.BaseURL == "" {
		c.BaseURL = "https://openrouter.ai/api/v1"
	}
	if c.DefaultTimeout == 0 {
		c.DefaultTimeout = 30 * time.Second
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}
	if c.RetryBackoff == 0 {
		c.RetryBackoff = 500 * time.Millisecond
	}
	if c.HTTPReferer == "" {
		c.HTTPReferer = "https://growth.app"
	}
	if c.XTitle == "" {
		c.XTitle = "growth"
	}
	if c.Models == nil {
		c.Models = make(map[ModelProfile]string)
	}
	for k, v := range DefaultModels {
		if _, ok := c.Models[k]; !ok {
			c.Models[k] = v
		}
	}
	if c.CostRates == nil {
		c.CostRates = make(map[string]CostRate)
	}
	for k, v := range DefaultCostRates {
		if _, ok := c.CostRates[k]; !ok {
			c.CostRates[k] = v
		}
	}
	if c.FallbackPolicy.Enabled && c.FallbackPolicy.MaxFailures == 0 {
		c.FallbackPolicy.MaxFailures = 2
	}
	return nil
}

// ModelID returns the concrete model ID for a profile.
func (c *Config) ModelID(p ModelProfile) (string, error) {
	id, ok := c.Models[p]
	if !ok {
		return "", fmt.Errorf("ai.Config: no model mapped for profile %q", p)
	}
	return id, nil
}

// ComputeCost returns the estimated USD cost for a request.
func (c *Config) ComputeCost(modelID string, promptTokens, completionTokens int) float64 {
	rate, ok := c.CostRates[modelID]
	if !ok {
		return 0
	}
	promptCost := float64(promptTokens) / 1000.0 * rate.PromptPer1K
	completionCost := float64(completionTokens) / 1000.0 * rate.CompletionPer1K
	return promptCost + completionCost
}
