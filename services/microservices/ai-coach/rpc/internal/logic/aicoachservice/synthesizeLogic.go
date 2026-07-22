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

type SynthesizeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSynthesizeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SynthesizeLogic {
	return &SynthesizeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Synthesize converts text into audio bytes via the configured TTS provider.
// Returns Unavailable if TTS is not configured.
func (l *SynthesizeLogic) Synthesize(in *aicoach.SynthesizeRequest) (*aicoach.SynthesizeResponse, error) {
	if l.svcCtx.TTS == nil {
		return nil, status.Error(codes.Unavailable, "text-to-speech is not configured")
	}
	if in.Text == "" {
		return nil, status.Error(codes.InvalidArgument, "text is required")
	}

	// TTS for a coaching response (~200 words) is well under 30s.
	ctx, cancel := context.WithTimeout(l.ctx, 30*time.Second)
	defer cancel()

	start := time.Now()
	audio, format, err := l.svcCtx.TTS.Synthesize(ctx, in.Text, speech.SynthesizeOptions{
		Voice: in.Voice,
	})
	if err != nil {
		l.Errorf("synthesize failed for user=%s text_len=%d after %v: %v",
			in.UserId, len(in.Text), time.Since(start), err)
		return nil, status.Errorf(codes.Internal, "synthesis failed: %v", err)
	}

	l.Infof("synthesize ok: user=%s text_len=%d format=%s bytes=%d elapsed=%v",
		in.UserId, len(in.Text), format, len(audio), time.Since(start))

	return &aicoach.SynthesizeResponse{
		Audio:  audio,
		Format: format,
	}, nil
}
