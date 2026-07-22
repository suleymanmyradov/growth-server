package speech

import "context"

// TranscribeOptions overrides per-call STT settings. Zero values fall back to
// the client's Config.STT defaults.
type TranscribeOptions struct {
	// Language is an ISO-639-1 code. Empty = auto-detect (provider default).
	Language string
	// Temperature (0–1). Lower = more deterministic. <=0 = provider default.
	Temperature float64
}

// TranscriptionResult is the output of a transcription call.
type TranscriptionResult struct {
	// Text is the full transcribed text.
	Text string
	// Language is the detected language (ISO-639-1), if the provider reports it.
	Language string
	// Duration is the audio duration in seconds, if the provider reports it.
	Duration float64
}

// STTClient converts audio into text. Implementations must be safe for
// concurrent use.
type STTClient interface {
	// Transcribe sends the given audio bytes (in the given format) to the STT
	// provider and returns the transcribed text.
	Transcribe(ctx context.Context, audio []byte, format AudioFormat, opts TranscribeOptions) (TranscriptionResult, error)
}
