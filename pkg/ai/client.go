package ai

import (
	"context"
	"fmt"
	"net/http"
	"time"

	openaimodel "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/zeromicro/go-zero/core/logx"
)

// Option configures a Client at construction time.
type Option func(*clientOptions)

type clientOptions struct {
	httpClient *http.Client
	quotaStore QuotaStore
}

// WithHTTPClient sets a custom HTTP client (useful for testing).
func WithHTTPClient(hc *http.Client) Option {
	return func(o *clientOptions) { o.httpClient = hc }
}

// WithQuotaStore sets the quota storage backend.
func WithQuotaStore(s QuotaStore) Option {
	return func(o *clientOptions) { o.quotaStore = s }
}

// Client is the single shared AI/LLM layer used by every microservice.
type Client interface {
	// Generate performs one-shot generation. Use for Phase 3C feedback,
	// Phase 4 review, safety classifier, notification copy.
	Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error)

	// Stream performs streaming generation. Use for Phase 5 conversational coach.
	Stream(ctx context.Context, req GenerateRequest) (StreamReader, error)

	// GenerateStructured wraps Generate + JSON schema validation.
	// Use for Phase 4 weekly review structured fields.
	GenerateStructured(ctx context.Context, req GenerateRequest, out any) error

	// RunAgent runs the model<->tool round-trip loop. Use for Phase 5
	// conversational coach and proactive agent.
	RunAgent(ctx context.Context, req AgentRequest) (AgentResponse, error)
}

// client implements Client.
type client struct {
	cfg    Config
	models map[ModelProfile]openaiModel // profile → eino model
	opts   clientOptions
}

// openaiModel wraps an Eino ChatModel with its model ID.
type openaiModel struct {
	modelID string
	chat    *openaimodel.ChatModel
}

// New creates a new Client from the given Config and options.
func New(cfg Config, opts ...Option) (Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("ai.New: %w", err)
	}

	o := clientOptions{}
	for _, opt := range opts {
		opt(&o)
	}

	httpClient := o.httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: cfg.DefaultTimeout}
	}

	// Always wrap the transport to inject OpenRouter analytics headers.
	origTransport := httpClient.Transport
	if origTransport == nil {
		origTransport = http.DefaultTransport
	}
	httpClient.Transport = &openRouterTransport{
		apiKey:      cfg.APIKey,
		httpReferer: cfg.HTTPReferer,
		xTitle:      cfg.XTitle,
		base:        origTransport,
	}

	models := make(map[ModelProfile]openaiModel, len(cfg.Models))
	for profile, modelID := range cfg.Models {
		cm, err := openaimodel.NewChatModel(context.Background(), &openaimodel.ChatModelConfig{
			APIKey:     cfg.APIKey,
			BaseURL:    cfg.BaseURL,
			Model:      modelID,
			HTTPClient: httpClient,
			Timeout:    cfg.DefaultTimeout,
		})
		if err != nil {
			return nil, fmt.Errorf("ai.New: create model for profile %q: %w", profile, err)
		}
		models[profile] = openaiModel{modelID: modelID, chat: cm}
	}

	return &client{cfg: cfg, models: models, opts: o}, nil
}

// modelFor returns the eino model for the given profile.
func (c *client) modelFor(p ModelProfile) (openaiModel, error) {
	m, ok := c.models[p]
	if !ok {
		return openaiModel{}, fmt.Errorf("ai: no model for profile %q: %w", p, ErrInvalidProfile)
	}
	return m, nil
}

// fallbackModel returns the fallback model if configured.
func (c *client) fallbackModel() (openaiModel, bool) {
	m, ok := c.models[ModelFallback]
	return m, ok
}

// checkQuota checks per-user and global quotas before making a call.
func (c *client) checkQuota(ctx context.Context, meta Metadata) error {
	if c.opts.quotaStore == nil {
		return nil
	}
	if meta.UserID != "" && c.cfg.Quota.UserDailyTokenCap > 0 {
		ok, err := c.opts.quotaStore.CheckUserQuota(ctx, meta.UserID, c.cfg.Quota.UserDailyTokenCap)
		if err != nil {
			logx.Errorf("ai: quota check error for user %s: %v", meta.UserID, err)
			return nil // fail open on quota store errors
		}
		if !ok {
			return &QuotaError{Limit: "user_daily", Cap: c.cfg.Quota.UserDailyTokenCap}
		}
	}
	if c.cfg.Quota.GlobalDailyCostCapUSD > 0 {
		ok, err := c.opts.quotaStore.CheckGlobalQuota(ctx, int64(c.cfg.Quota.GlobalDailyCostCapUSD*1e6))
		if err != nil {
			logx.Errorf("ai: global quota check error: %v", err)
			return nil
		}
		if !ok {
			return &QuotaError{Limit: "global_daily", Cap: int64(c.cfg.Quota.GlobalDailyCostCapUSD * 1e6)}
		}
	}
	return nil
}

// recordUsage records token usage for quota tracking.
func (c *client) recordUsage(ctx context.Context, meta Metadata, usage Usage, costUSD float64) {
	if c.opts.quotaStore == nil {
		return
	}
	if meta.UserID != "" && c.cfg.Quota.UserDailyTokenCap > 0 {
		if err := c.opts.quotaStore.IncrUserTokens(ctx, meta.UserID, int64(usage.TotalTokens)); err != nil {
			logx.Errorf("ai: record user tokens error: %v", err)
		}
	}
	if c.cfg.Quota.GlobalDailyCostCapUSD > 0 {
		if err := c.opts.quotaStore.IncrGlobalCost(ctx, int64(costUSD*1e6)); err != nil {
			logx.Errorf("ai: record global cost error: %v", err)
		}
	}
}

// logCall logs every call with model, profile, feature, tokens, latency, cost, error.
// Never logs message contents at info level.
func (c *client) logCall(profile ModelProfile, modelID string, meta Metadata, usage Usage, latencyMS int64, costUSD float64, err error) {
	if err != nil {
		logx.Infof("ai call: profile=%s model=%s feature=%s user=%s latency_ms=%d cost_usd=%.6f error=%v",
			profile, modelID, meta.Feature, meta.UserID, latencyMS, costUSD, err)
		return
	}
	logx.Infof("ai call: profile=%s model=%s feature=%s user=%s prompt_tokens=%d completion_tokens=%d latency_ms=%d cost_usd=%.6f",
		profile, modelID, meta.Feature, meta.UserID, usage.PromptTokens, usage.CompletionTokens, latencyMS, costUSD)
}

// openRouterTransport injects OpenRouter analytics headers into every request.
type openRouterTransport struct {
	apiKey      string
	httpReferer string
	xTitle      string
	base        http.RoundTripper
}

func (t *openRouterTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	if t.httpReferer != "" {
		req.Header.Set("HTTP-Referer", t.httpReferer)
	}
	if t.xTitle != "" {
		req.Header.Set("X-Title", t.xTitle)
	}
	return t.base.RoundTrip(req)
}

// einoModelOptions builds Eino model.Option list from our request fields.
func einoModelOptions(req GenerateRequest) []model.Option {
	var opts []model.Option
	if req.Temperature != nil {
		opts = append(opts, model.WithTemperature(*req.Temperature))
	}
	if req.MaxTokens != nil {
		opts = append(opts, model.WithMaxTokens(*req.MaxTokens))
	}
	return opts
}

// buildToolInfos extracts Eino ToolInfo from our Tool slice.
func buildToolInfos(tools []Tool) []*schema.ToolInfo {
	infos := make([]*schema.ToolInfo, len(tools))
	for i, t := range tools {
		infos[i] = t.Info()
	}
	return infos
}

// callGenerate is the internal method that calls the Eino model with retry.
func (c *client) callGenerate(ctx context.Context, m openaiModel, msgs []*schema.Message, opts []model.Option) (*schema.Message, error) {
	var result *schema.Message
	err := c.withRetry(ctx, m.modelID, func(ctx context.Context) error {
		var genErr error
		result, genErr = m.chat.Generate(ctx, msgs, opts...)
		return genErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// callGenerateWithTools calls the Eino model with tools bound.
func (c *client) callGenerateWithTools(ctx context.Context, m openaiModel, msgs []*schema.Message, toolInfos []*schema.ToolInfo, opts []model.Option) (*schema.Message, error) {
	tooledModel, err := m.chat.WithTools(toolInfos)
	if err != nil {
		return nil, fmt.Errorf("ai: bind tools: %w", err)
	}

	var result *schema.Message
	retryErr := c.withRetry(ctx, m.modelID, func(ctx context.Context) error {
		var genErr error
		result, genErr = tooledModel.Generate(ctx, msgs, opts...)
		return genErr
	})
	if retryErr != nil {
		return nil, retryErr
	}
	return result, nil
}

// nowMS returns current unix milliseconds.
func nowMS() int64 {
	return time.Now().UnixMilli()
}
