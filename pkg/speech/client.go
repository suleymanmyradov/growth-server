package speech

import (
	"encoding/base64"
	"fmt"
)

// base64StdEncoding is the standard base64 encoding used for audio payloads.
var base64StdEncoding = base64.StdEncoding

// Clients bundles the optional STT and TTS clients built from a Config. Either
// may be nil when the corresponding model is empty (feature disabled).
type Clients struct {
	STT STTClient
	TTS TTSClient
}

// New builds the STT and TTS clients from the given Config. If the config is
// empty (no API key), both clients are nil and no error is returned — speech
// is an opt-in feature.
func New(cfg Config) (Clients, error) {
	if err := cfg.Validate(); err != nil {
		return Clients{}, fmt.Errorf("speech.New: %w", err)
	}
	if cfg.APIKey == "" {
		return Clients{}, nil
	}
	stt, tts := newClients(cfg)
	return Clients{STT: stt, TTS: tts}, nil
}
