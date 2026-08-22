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

// geminiSTT implements STTClient against Google's Gemini generateContent API.
// Gemini multimodal models (Flash-Lite, Flash, Pro) accept audio as inline_data
// and can transcribe it with a text prompt. This reuses a standard Google AI
// Studio API key (generativelanguage.googleapis.com) — no GCP service account
// needed.
type geminiSTT struct {
	apiKey   string
	http     *http.Client
	model    string
	language string // configured default language (ISO-639-1), empty = auto-detect
	// baseURL defaults to https://generativelanguage.googleapis.com/v1beta.
	baseURL string
}

// newGeminiSTT returns nil if STT is not configured (no model).
func newGeminiSTT(cfg Config) STTClient {
	if cfg.STT.Model == "" {
		return nil
	}
	base := cfg.BaseURL
	if base == "" {
		base = "https://generativelanguage.googleapis.com/v1beta"
	}
	return &geminiSTT{
		apiKey:   cfg.APIKey,
		http:     newGeminiHTTPClient(cfg),
		model:    cfg.STT.Model,
		language: cfg.STT.Language,
		baseURL:  base,
	}
}

// newGeminiHTTPClient builds a clean HTTP client without the OpenRouter
// authTransport. Gemini authenticates via ?key= query parameter, not a
// Bearer header — the shared authTransport's Authorization header causes
// Gemini to reject the request with API_KEY_SERVICE_BLOCKED.
func newGeminiHTTPClient(cfg Config) *http.Client {
	transport := http.DefaultTransport
	if transport == nil {
		transport = &http.Transport{}
	}
	return &http.Client{
		Transport: transport,
		Timeout:   cfg.DefaultTimeout,
	}
}

// --- Gemini API types (only what we need for transcription) ---

type geminiRequest struct {
	Contents         []geminiContent `json:"contents"`
	GenerationConfig geminiGenConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

type geminiPart struct {
	Text       string        `json:"text,omitempty"`
	InlineData *geminiInline `json:"inlineData,omitempty"`
}

type geminiInline struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type geminiGenConfig struct {
	Temperature float64 `json:"temperature,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	PromptFeedback *struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback,omitempty"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// geminiMime maps our AudioFormat to the MIME types Gemini accepts.
func geminiMime(format AudioFormat) string {
	switch format {
	case AudioWAV:
		return "audio/wav"
	case AudioMP3:
		return "audio/mp3"
	case AudioFLAC:
		return "audio/flac"
	case AudioM4A:
		return "audio/m4a"
	case AudioOGG:
		return "audio/ogg"
	case AudioWebM:
		return "audio/webm"
	case AudioAAC:
		return "audio/x-aac"
	default:
		return ""
	}
}

func (c *geminiSTT) Transcribe(ctx context.Context, audio []byte, format AudioFormat, opts TranscribeOptions) (TranscriptionResult, error) {
	if len(audio) == 0 {
		return TranscriptionResult{}, fmt.Errorf("speech: empty audio payload")
	}
	mime := geminiMime(format)
	if mime == "" {
		return TranscriptionResult{}, fmt.Errorf("%w: %s", ErrUnsupportedFormat, format)
	}

	prompt := "Transcribe the following audio. Output only the transcribed text, with no additional commentary or formatting."
	if lang := firstNonEmpty(opts.Language, c.language); lang != "" {
		prompt = fmt.Sprintf("Transcribe the following audio in %s. Output only the transcribed text, with no additional commentary or formatting.", lang)
	}

	body := geminiRequest{
		Contents: []geminiContent{{
			Role: "user",
			Parts: []geminiPart{
				{Text: prompt},
				{InlineData: &geminiInline{
					MimeType: mime,
					Data:     base64Encode(audio),
				}},
			},
		}},
		GenerationConfig: geminiGenConfig{
			Temperature: 0,
		},
	}
	if opts.Temperature > 0 {
		body.GenerationConfig.Temperature = opts.Temperature
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return TranscriptionResult{}, fmt.Errorf("speech: marshal gemini stt request: %w", err)
	}

	// Pass the API key as a query parameter. Gemini uses ?key= for API key
	// auth. We don't use the shared authTransport's Authorization: Bearer
	// header because Gemini rejects Bearer tokens with
	// API_KEY_SERVICE_BLOCKED — the geminiSTT client gets a clean HTTP
	// client (no authTransport) from newGeminiHTTPClient.
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, c.model, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return TranscriptionResult{}, fmt.Errorf("speech: new gemini stt request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return TranscriptionResult{}, fmt.Errorf("speech: gemini stt request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return TranscriptionResult{}, &ProviderError{Provider: "gemini", Status: resp.StatusCode, Body: truncate(string(respBody), 512)}
	}

	var out geminiResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return TranscriptionResult{}, fmt.Errorf("speech: decode gemini stt response: %w", err)
	}
	if out.Error != nil {
		return TranscriptionResult{}, fmt.Errorf("speech: gemini error %d: %s", out.Error.Code, out.Error.Message)
	}
	if out.PromptFeedback != nil && out.PromptFeedback.BlockReason != "" {
		return TranscriptionResult{}, fmt.Errorf("speech: gemini blocked: %s", out.PromptFeedback.BlockReason)
	}

	var text string
	if len(out.Candidates) > 0 {
		for _, part := range out.Candidates[0].Content.Parts {
			text += part.Text
		}
	}
	return TranscriptionResult{
		Text: strings.TrimSpace(text),
	}, nil
}
