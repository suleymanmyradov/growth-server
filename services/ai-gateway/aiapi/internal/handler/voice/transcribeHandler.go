package voice

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/aicoachservice"
	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TranscribeResponse is the JSON body returned by POST /personalization/transcribe.
type TranscribeResponse struct {
	Text     string  `json:"text"`
	Language string  `json:"language,omitempty"`
	Duration float64 `json:"duration,omitempty"`
}

// maxTranscribeBytes is the upload cap for a single dictate clip (25 MB,
// matching the OpenAI Whisper limit).
const maxTranscribeBytes = 25 << 20

// TranscribeHandler accepts a multipart audio upload (field "audio") and
// returns the transcribed text. Used by the dictate mic button in the
// composer: the browser captures audio via MediaRecorder, uploads it, and
// inserts the returned text into the message input.
func TranscribeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := principal.PrincipalFrom(r.Context())
		if !ok {
			errors.HandleGrpcError(w, status.Error(codes.Unauthenticated, "not authenticated"))
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxTranscribeBytes)
		if err := r.ParseMultipartForm(maxTranscribeBytes); err != nil {
			errors.WriteParseError(w, err)
			return
		}

		file, header, err := r.FormFile("audio")
		if err != nil {
			errors.WriteParseError(w, err)
			return
		}
		defer func() { _ = file.Close() }()

		// Derive the audio format from the filename extension or the
		// Content-Type. The STT provider needs a bare format string
		// (e.g. "webm"), not a MIME type.
		format := audioFormatFromHeader(header.Filename, header.Header.Get("Content-Type"))
		if format == "" {
			errors.WriteParseError(w, status.Error(codes.InvalidArgument, "could not determine audio format from filename or content-type"))
			return
		}

		// Read the audio bytes. MaxBytesReader above caps the total request
		// body, so io.ReadAll cannot exceed the limit.
		audio, err := io.ReadAll(file)
		if err != nil {
			errors.WriteParseError(w, err)
			return
		}

		language := r.FormValue("language")

		resp, err := svcCtx.AICoachRpc.AICoachService.Transcribe(r.Context(), &aicoachservice.TranscribeRequest{
			UserId:   p.UserID,
			Audio:    audio,
			Format:   format,
			Language: language,
		})
		if err != nil {
			errors.HandleGrpcError(w, err)
			return
		}

		httpx.OkJsonCtx(r.Context(), w, TranscribeResponse{
			Text:     resp.Text,
			Language: resp.Language,
			Duration: resp.Duration,
		})
	}
}

// audioFormatFromHeader derives the bare audio format string expected by the
// STT provider from either the filename extension or the MIME content type.
func audioFormatFromHeader(filename, contentType string) string {
	// Prefer the filename extension — MediaRecorder sets a generic
	// audio/webm or audio/ogg MIME type that maps cleanly, but a .m4a
	// upload from iOS Safari reports audio/mp4 which is ambiguous.
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	switch ext {
	case "wav", "mp3", "flac", "m4a", "ogg", "webm", "aac":
		return ext
	}

	switch strings.ToLower(contentType) {
	case "audio/wav", "audio/wave", "audio/x-wav":
		return "wav"
	case "audio/mpeg", "audio/mp3":
		return "mp3"
	case "audio/flac":
		return "flac"
	case "audio/mp4", "audio/m4a", "audio/x-m4a":
		return "m4a"
	case "audio/ogg", "application/ogg":
		return "ogg"
	case "audio/webm":
		return "webm"
	case "audio/aac":
		return "aac"
	}
	return ""
}
