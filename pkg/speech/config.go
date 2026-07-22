package speech

import (
	"fmt"
	"time"
)

// AudioFormat identifies the container/codec of an audio payload. Values match
// the strings accepted by OpenAI-compatible transcription endpoints.
type AudioFormat string

const (
	AudioWAV  AudioFormat = "wav"
	AudioMP3  AudioFormat = "mp3"
	AudioFLAC AudioFormat = "flac"
	AudioM4A  AudioFormat = "m4a"
	AudioOGG  AudioFormat = "ogg"
	AudioWebM AudioFormat = "webm"
	AudioAAC  AudioFormat = "aac"
)

// Config holds all configuration for the speech (STT + TTS) layer. It is
// loadable from go-zero YAML configs. Both STT and TTS share a single API key
// and base URL (the common case for OpenRouter / OpenAI), but each can be
// independently disabled by leaving its Model empty.
type Config struct {
	// APIKey is the provider API key (OpenRouter or OpenAI). Never logged.
	APIKey string `json:"api_key,optional" secret:"true"`
	// BaseURL defaults to https://openrouter.ai/api/v1. Use
	// https://api.openai.com/v1 for direct OpenAI access.
	BaseURL string `json:"base_url,optional"`
	// HTTPReferer / XTitle are OpenRouter analytics headers (ignored by OpenAI).
	HTTPReferer string `json:"http_referer,optional"`
	XTitle      string `json:"x_title,optional"`
	// DefaultTimeout per request. Defaults to 60s (audio files can be large).
	DefaultTimeout time.Duration `json:"default_timeout,optional"`
	// STT configures speech-to-text. Optional: if Model is empty, STT is
	// disabled and NewSTTClient returns nil.
	STT STTConfig `json:"stt,optional"`
	// TTS configures text-to-speech. Optional: if Model is empty, TTS is
	// disabled and NewTTSClient returns nil.
	TTS TTSConfig `json:"tts,optional"`
}

// STTConfig configures the speech-to-text provider.
type STTConfig struct {
	// Provider selects the implementation. Currently supported: "openrouter"
	// (default, OpenAI-compatible). Future: "deepgram", "assemblyai".
	Provider string `json:",optional"`
	// Model is the STT model ID, e.g. "openai/whisper-1" (OpenRouter) or
	// "whisper-1" (OpenAI direct). Empty disables STT.
	Model string `json:",optional"`
	// Language is an ISO-639-1 code (e.g. "en"). Auto-detected if empty.
	Language string `json:",optional"`
}

// TTSConfig configures the text-to-speech provider.
type TTSConfig struct {
	// Provider selects the implementation. Currently supported: "openrouter"
	// (default, OpenAI-compatible). Future: "elevenlabs", "cartesia".
	Provider string `json:",optional"`
	// Model is the TTS model ID, e.g. "openai/gpt-4o-mini-tts" (OpenRouter) or
	// "gpt-4o-mini-tts" (OpenAI direct). Empty disables TTS.
	Model string `json:",optional"`
	// Voice is the provider-specific voice identifier (e.g. "alloy", "nova").
	Voice string `json:",optional"`
	// Format is the output audio format: "mp3" (default) or "pcm".
	Format string `json:",optional"`
	// Speed is the playback speed multiplier (0.5–2.0). Default 1.0.
	Speed float64 `json:",optional"`
}

// Validate applies defaults and returns an error if a configured provider is
// unknown. An empty Config (no API key, no models) is valid and yields nil
// clients — speech is an opt-in feature.
func (c *Config) Validate() error {
	if c.APIKey == "" {
		// Nothing configured; both clients will be nil. That's fine.
		return nil
	}
	if c.BaseURL == "" {
		c.BaseURL = "https://openrouter.ai/api/v1"
	}
	if c.HTTPReferer == "" {
		c.HTTPReferer = "https://growth.app"
	}
	if c.XTitle == "" {
		c.XTitle = "growth"
	}
	if c.DefaultTimeout == 0 {
		c.DefaultTimeout = 60 * time.Second
	}
	if c.STT.Provider == "" && c.STT.Model != "" {
		c.STT.Provider = "openrouter"
	}
	if c.TTS.Provider == "" && c.TTS.Model != "" {
		c.TTS.Provider = "openrouter"
	}
	if c.TTS.Format == "" && c.TTS.Model != "" {
		c.TTS.Format = "mp3"
	}
	if c.TTS.Speed == 0 && c.TTS.Model != "" {
		c.TTS.Speed = 1.0
	}
	for _, p := range []string{c.STT.Provider, c.TTS.Provider} {
		switch p {
		case "", "openrouter":
		default:
			return fmt.Errorf("speech.Config: unsupported provider %q (supported: openrouter)", p)
		}
	}
	return nil
}
