package aicoachservicelogic

import (
	"context"
	"math"
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

	// Per-user daily voice quota (audio seconds). STT is provider-billed —
	// without a dedicated cap, repeated transcription bypasses the token
	// quota entirely. Fails closed: a cap with a missing/erroring store
	// rejects rather than silently allowing unlimited transcription.
	if cap := l.svcCtx.Config.AI.Quota.UserDailyVoiceSecondsCap; cap > 0 && in.UserId != "" {
		if l.svcCtx.QuotaStore == nil {
			l.Errorf("voice quota cap configured but quota store unavailable; failing closed: user=%s", in.UserId)
			return nil, status.Error(codes.ResourceExhausted, "daily voice limit unavailable — please try again later")
		}
		ok, err := l.svcCtx.QuotaStore.CheckUserVoiceQuota(l.ctx, in.UserId, cap)
		if err != nil {
			l.Errorf("voice quota check error, failing closed: user=%s err=%v", in.UserId, err)
			return nil, status.Error(codes.ResourceExhausted, "daily voice limit unavailable — please try again later")
		}
		if !ok {
			return nil, status.Error(codes.ResourceExhausted, "daily voice limit reached")
		}
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

	// Record billed audio seconds against the daily voice quota. When the
	// provider doesn't report duration, the wall-clock STT call is the best
	// available proxy — still bounded, still metered.
	if l.svcCtx.QuotaStore != nil && in.UserId != "" {
		seconds := result.Duration
		if seconds <= 0 {
			seconds = time.Since(start).Seconds()
		}
		if seconds > 0 {
			if err := l.svcCtx.QuotaStore.IncrUserVoiceSeconds(l.ctx, in.UserId, int64(math.Ceil(seconds))); err != nil {
				l.Errorf("voice quota increment failed: user=%s err=%v", in.UserId, err)
			}
		}
	}

	l.Infof("transcribe ok: user=%s format=%s bytes=%d duration=%.1fs text_len=%d elapsed=%v",
		in.UserId, in.Format, len(in.Audio), result.Duration, len(result.Text), time.Since(start))

	return &aicoach.TranscribeResponse{
		Text:     result.Text,
		Language: result.Language,
		Duration: result.Duration,
	}, nil
}
