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

// requiredProfiles lists every ModelProfile that must be mapped in Config.Models.
// If any is missing, Validate returns an error so no AI-using service can start
// without an explicit model assignment.
var requiredProfiles = []ModelProfile{
	ModelCheap,
	ModelCheapLong,
	ModelClassifier,
	ModelChat,
	ModelFallback,
}

// CostRate describes the per-1K-token cost in USD for a model.
type CostRate struct {
	PromptPer1K     float64
	CompletionPer1K float64
}

// FallbackPolicy controls automatic retry with a fallback model.
type FallbackPolicy struct {
	// Enabled turns on fallback behaviour.
	Enabled bool `json:"enabled,optional"`
	// MaxFailures is the number of failures on the primary model before
	// retrying once with the fallback model.
	MaxFailures int `json:"max_failures,optional"`
}

// Config holds all configuration for the AI client. It is loadable from
// go-zero YAML configs via conf.MustLoad.
type Config struct {
	// APIKey is the OpenRouter API key (env: OPENROUTER_API_KEY). Never logged.
	APIKey string `json:"api_key"`
	// BaseURL defaults to https://openrouter.ai/api/v1.
	BaseURL string `json:"base_url,optional"`
	// Models maps profile name (string key, e.g. "cheap", "chat") to model ID.
	// Required: every profile in requiredProfiles must be present or Validate
	// will reject the config and no AI-using service will start.
	Models map[string]string `json:"models"`
	// CostRates maps model ID to per-1K-token pricing. Optional; unknown models
	// default to zero cost.
	CostRates map[string]CostRate `json:"cost_rates,optional"`
	// DefaultTimeout per request.
	DefaultTimeout time.Duration `json:"default_timeout,optional"`
	// MaxRetries for transient errors (429 / 5xx).
	MaxRetries int `json:"max_retries,optional"`
	// RetryBackoff is the initial backoff duration; doubles each retry.
	RetryBackoff time.Duration `json:"retry_backoff,optional"`
	// HTTPReferer is the OpenRouter analytics HTTP-Referer header.
	HTTPReferer string `json:"http_referer,optional"`
	// XTitle is the OpenRouter analytics X-Title header.
	XTitle string `json:"x_title,optional"`
	// FallbackPolicy controls automatic retry with a fallback model.
	FallbackPolicy FallbackPolicy `json:"fallback_policy,optional"`
	// LogPrompts enables logging of prompt/completion contents at info level.
	// Only use in development; never enable in production.
	LogPrompts bool `json:"log_prompts,optional"`
	// Quota configuration (optional; requires Redis).
	Quota QuotaConfig `json:"quota,optional"`
}

// QuotaConfig controls per-user and global daily spend caps.
type QuotaConfig struct {
	// RedisAddr is the Redis address for quota tracking.
	RedisAddr string `json:"redis_addr,optional"`
	// RedisPassword for Redis auth.
	RedisPassword string `json:"redis_password,optional"`
	// RedisDB selects the Redis database number.
	RedisDB int `json:"redis_db,optional"`
	// UserDailyTokenCap is the max tokens a single user can consume per day.
	// 0 means unlimited.
	UserDailyTokenCap int64 `json:"user_daily_token_cap,optional"`
	// GlobalDailyCostCapUSD is the max total spend across all users per day.
	// 0 means unlimited.
	GlobalDailyCostCapUSD float64 `json:"global_daily_cost_cap_usd,optional"`
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
		c.Models = make(map[string]string)
	}
	// Every required profile must be explicitly mapped in config. No defaults.
	var missing []string
	for _, p := range requiredProfiles {
		if id, ok := c.Models[string(p)]; !ok || id == "" {
			missing = append(missing, string(p))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("ai.Config: Models is missing required profiles: %v (set them under AI.models in the YAML config)", missing)
	}
	if c.CostRates == nil {
		c.CostRates = make(map[string]CostRate)
	}
	if c.FallbackPolicy.Enabled && c.FallbackPolicy.MaxFailures == 0 {
		c.FallbackPolicy.MaxFailures = 2
	}
	return nil
}

// ModelID returns the concrete model ID for a profile.
func (c *Config) ModelID(p ModelProfile) (string, error) {
	id, ok := c.Models[string(p)]
	if !ok || id == "" {
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
