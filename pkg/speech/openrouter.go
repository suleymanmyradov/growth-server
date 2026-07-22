package speech

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// openRouterSTT implements STTClient against the OpenAI-compatible
// /audio/transcriptions endpoint (OpenRouter, OpenAI direct, Groq, etc.).
type openRouterSTT struct {
	cfg    Config
	http   *http.Client
	model  string
}

// openRouterTTS implements TTSClient against the OpenAI-compatible
// /audio/speech endpoint.
type openRouterTTS struct {
	cfg   Config
	http  *http.Client
	model string
}

// newOpenRouterSTT returns nil if STT is not configured (no model).
func newOpenRouterSTT(cfg Config, hc *http.Client) STTClient {
	if cfg.STT.Model == "" {
		return nil
	}
	return &openRouterSTT{cfg: cfg, http: hc, model: cfg.STT.Model}
}

// newOpenRouterTTS returns nil if TTS is not configured (no model).
func newOpenRouterTTS(cfg Config, hc *http.Client) TTSClient {
	if cfg.TTS.Model == "" {
		return nil
	}
	return &openRouterTTS{cfg: cfg, http: hc, model: cfg.TTS.Model}
}

// --- STT ---

// sttJSONRequest is the JSON body for the OpenAI-compatible transcriptions
// endpoint (base64-encoded audio). Used by OpenRouter.
type sttJSONRequest struct {
	Model       string          `json:"model"`
	InputAudio  sttInputAudio   `json:"input_audio"`
	Language    string          `json:"language,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

type sttInputAudio struct {
	Data   string `json:"data"`   // base64-encoded audio bytes (no data URI prefix)
	Format string `json:"format"` // wav, mp3, flac, m4a, ogg, webm, aac
}

// sttJSONResponse is the default JSON response from /audio/transcriptions.
type sttJSONResponse struct {
	Text     string  `json:"text"`
	Language string  `json:"language,omitempty"`
	Duration float64 `json:"duration,omitempty"`
}

func (c *openRouterSTT) Transcribe(ctx context.Context, audio []byte, format AudioFormat, opts TranscribeOptions) (TranscriptionResult, error) {
	if len(audio) == 0 {
		return TranscriptionResult{}, fmt.Errorf("speech: empty audio payload")
	}
	if format == "" {
		return TranscriptionResult{}, ErrUnsupportedFormat
	}

	body := sttJSONRequest{
		Model: c.model,
		InputAudio: sttInputAudio{
			Data:   base64Encode(audio),
			Format: string(format),
		},
	}
	if lang := firstNonEmpty(opts.Language, c.cfg.STT.Language); lang != "" {
		body.Language = lang
	}
	if opts.Temperature > 0 {
		body.Temperature = opts.Temperature
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return TranscriptionResult{}, fmt.Errorf("speech: marshal stt request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/audio/transcriptions", bytes.NewReader(payload))
	if err != nil {
		return TranscriptionResult{}, fmt.Errorf("speech: new stt request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return TranscriptionResult{}, fmt.Errorf("speech: stt request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return TranscriptionResult{}, &ProviderError{Provider: "openrouter", Status: resp.StatusCode, Body: truncate(string(respBody), 512)}
	}

	var out sttJSONResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return TranscriptionResult{}, fmt.Errorf("speech: decode stt response: %w", err)
	}
	return TranscriptionResult{
		Text:     strings.TrimSpace(out.Text),
		Language: out.Language,
		Duration: out.Duration,
	}, nil
}

// --- TTS ---

// ttsRequest is the JSON body for the OpenAI-compatible /audio/speech endpoint.
type ttsRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
}

func (c *openRouterTTS) Synthesize(ctx context.Context, text string, opts SynthesizeOptions) ([]byte, string, error) {
	if text == "" {
		return nil, "", fmt.Errorf("speech: empty text for tts")
	}

	format := firstNonEmpty(opts.Format, c.cfg.TTS.Format, "mp3")
	body := ttsRequest{
		Model:          c.model,
		Input:          text,
		Voice:          firstNonEmpty(opts.Voice, c.cfg.TTS.Voice, "alloy"),
		ResponseFormat: format,
	}
	if speed := opts.Speed; speed > 0 {
		body.Speed = speed
	} else if c.cfg.TTS.Speed > 0 {
		body.Speed = c.cfg.TTS.Speed
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("speech: marshal tts request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/audio/speech", bytes.NewReader(payload))
	if err != nil {
		return nil, "", fmt.Errorf("speech: new tts request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("speech: tts request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, "", &ProviderError{Provider: "openrouter", Status: resp.StatusCode, Body: truncate(string(respBody), 512)}
	}

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("speech: read tts audio: %w", err)
	}
	return audio, format, nil
}

// --- shared helpers ---

// authTransport injects the bearer token and OpenRouter analytics headers.
type authTransport struct {
	apiKey      string
	httpReferer string
	xTitle      string
	base        http.RoundTripper
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	if t.httpReferer != "" {
		req.Header.Set("HTTP-Referer", t.httpReferer)
	}
	if t.xTitle != "" {
		req.Header.Set("X-Title", t.xTitle)
	}
	return t.base.RoundTrip(req)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// base64Encode avoids importing encoding/base64 at the top to keep the import
// block tidy; it's a thin wrapper.
func base64Encode(b []byte) string {
	return base64StdEncoding.EncodeToString(b)
}

// timeoutTransport wraps the base transport with the configured timeout. We
// don't set a timeout on http.Client directly because that includes response
// body reading; instead we rely on the caller's context deadline plus a
// transport-level idle-conn timeout.
func newHTTPClient(cfg Config) *http.Client {
	transport := http.DefaultTransport
	if transport == nil {
		transport = &http.Transport{}
	}
	return &http.Client{
		Transport: &authTransport{
			apiKey:      cfg.APIKey,
			httpReferer: cfg.HTTPReferer,
			xTitle:      cfg.XTitle,
			base:        transport,
		},
		Timeout: cfg.DefaultTimeout,
	}
}

// newClients builds the STT and TTS clients for the configured provider. Both
// may be nil if the corresponding model is empty.
func newClients(cfg Config) (STTClient, TTSClient) {
	hc := newHTTPClient(cfg)
	var stt STTClient
	var tts TTSClient
	switch cfg.STT.Provider {
	case "", "openrouter":
		stt = newOpenRouterSTT(cfg, hc)
	}
	switch cfg.TTS.Provider {
	case "", "openrouter":
		tts = newOpenRouterTTS(cfg, hc)
	}
	return stt, tts
}


