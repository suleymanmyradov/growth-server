package aicoachservicelogic

import (
	"context"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/speech"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TranscribeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTranscribeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TranscribeLogic {
	return &TranscribeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Transcribe converts audio bytes into text via the configured STT provider.
// Returns Unavailable if STT is not configured.
func (l *TranscribeLogic) Transcribe(in *aicoach.TranscribeRequest) (*aicoach.TranscribeResponse, error) {
	if l.svcCtx.STT == nil {
		return nil, status.Error(codes.Unavailable, "speech-to-text is not configured")
	}
	if len(in.Audio) == 0 {
		return nil, status.Error(codes.InvalidArgument, "audio payload is empty")
	}
	if in.Format == "" {
		return nil, status.Error(codes.InvalidArgument, "audio format is required")
	}

	// STT can take a few seconds for longer clips; allow up to 30s.
	ctx, cancel := context.WithTimeout(l.ctx, 30*time.Second)
	defer cancel()

	start := time.Now()
	result, err := l.svcCtx.STT.Transcribe(ctx, in.Audio, speech.AudioFormat(in.Format), speech.TranscribeOptions{
		Language: in.Language,
	})
	if err != nil {
		l.Errorf("transcribe failed for user=%s format=%s bytes=%d after %v: %v",
			in.UserId, in.Format, len(in.Audio), time.Since(start), err)
		return nil, status.Errorf(codes.Internal, "transcription failed: %v", err)
	}

	l.Infof("transcribe ok: user=%s format=%s bytes=%d duration=%.1fs text_len=%d elapsed=%v",
		in.UserId, in.Format, len(in.Audio), result.Duration, len(result.Text), time.Since(start))

	return &aicoach.TranscribeResponse{
		Text:     result.Text,
		Language: result.Language,
		Duration: result.Duration,
	}, nil
}
