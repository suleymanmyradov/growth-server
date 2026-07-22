package speech

import "context"

// SynthesizeOptions overrides per-call TTS settings. Zero values fall back to
// the client's Config.TTS defaults.
type SynthesizeOptions struct {
	// Voice is the provider-specific voice identifier. Empty = config default.
	Voice string
	// Format is the output audio format: "mp3" or "pcm". Empty = config default.
	Format string
	// Speed is the playback speed multiplier (0.5–2.0). <=0 = config default.
	Speed float64
}

// TTSClient converts text into audio. Implementations must be safe for
// concurrent use.
type TTSClient interface {
	// Synthesize sends the given text to the TTS provider and returns the
	// synthesized audio bytes in the requested format. The format string is
	// returned alongside the bytes so the caller knows how to play it.
	Synthesize(ctx context.Context, text string, opts SynthesizeOptions) (audio []byte, format string, err error)
}
