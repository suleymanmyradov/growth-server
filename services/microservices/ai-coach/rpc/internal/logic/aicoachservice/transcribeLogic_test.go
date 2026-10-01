package aicoachservicelogic

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/speech"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"
)

// mockSTT returns a fixed transcription and records call count.
type mockSTT struct {
	calls    atomic.Int32
	duration float64
	err      error
}

func (m *mockSTT) Transcribe(_ context.Context, _ []byte, _ speech.AudioFormat, _ speech.TranscribeOptions) (speech.TranscriptionResult, error) {
	m.calls.Add(1)
	if m.err != nil {
		return speech.TranscriptionResult{}, m.err
	}
	return speech.TranscriptionResult{Text: "hello", Language: "en", Duration: m.duration}, nil
}

// mockVoiceQuotaStore records voice-quota interactions for tests.
type mockVoiceQuotaStore struct {
	checkOK  bool
	checkErr error
	seconds  atomic.Int64
	calls    atomic.Int32
}

func (m *mockVoiceQuotaStore) CheckUserQuota(_ context.Context, _ string, _ int64) (bool, error) {
	return true, nil
}
func (m *mockVoiceQuotaStore) UserDailyTokens(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (m *mockVoiceQuotaStore) IncrUserTokens(_ context.Context, _ string, _ int64) error {
	return nil
}
func (m *mockVoiceQuotaStore) CheckGlobalQuota(_ context.Context, _ int64) (bool, error) {
	return true, nil
}
func (m *mockVoiceQuotaStore) IncrGlobalCost(_ context.Context, _ int64) error { return nil }
func (m *mockVoiceQuotaStore) CheckUserVoiceQuota(_ context.Context, _ string, _ int64) (bool, error) {
	m.calls.Add(1)
	return m.checkOK, m.checkErr
}
func (m *mockVoiceQuotaStore) IncrUserVoiceSeconds(_ context.Context, _ string, s int64) error {
	m.seconds.Add(s)
	return nil
}

func transcribeSvcCtx(stt *mockSTT, store ai.QuotaStore, voiceCap int64) *svc.ServiceContext {
	return &svc.ServiceContext{
		STT:        stt,
		QuotaStore: store,
		Config: config.Config{
			AI: ai.Config{Quota: ai.QuotaConfig{UserDailyVoiceSecondsCap: voiceCap}},
		},
	}
}

func transcribeReq() *aicoach.TranscribeRequest {
	return &aicoach.TranscribeRequest{UserId: "user-1", Audio: []byte("audio-bytes"), Format: "webm"}
}

func TestTranscribe_NoCap_NoQuotaCheck(t *testing.T) {
	stt := &mockSTT{duration: 4.2}
	store := &mockVoiceQuotaStore{}
	logic := NewTranscribeLogic(context.Background(), transcribeSvcCtx(stt, store, 0))

	resp, err := logic.Transcribe(transcribeReq())
	require.NoError(t, err)
	assert.Equal(t, "hello", resp.Text)
	assert.Equal(t, int32(0), store.calls.Load(), "no cap → no quota check")
	// Usage is still recorded when a store exists (even with no cap) so an
	// operator who later sets a cap sees accurate counters.
	assert.Equal(t, int64(5), store.seconds.Load()) // ceil(4.2)
}

func TestTranscribe_UnderCap_RecordsSeconds(t *testing.T) {
	stt := &mockSTT{duration: 4.2}
	store := &mockVoiceQuotaStore{checkOK: true}
	logic := NewTranscribeLogic(context.Background(), transcribeSvcCtx(stt, store, 1800))

	resp, err := logic.Transcribe(transcribeReq())
	require.NoError(t, err)
	assert.Equal(t, "hello", resp.Text)
	assert.Equal(t, int32(1), store.calls.Load())
	assert.Equal(t, int64(5), store.seconds.Load(), "ceil(4.2s) billed")
}

func TestTranscribe_OverCap_RejectedBeforeSTT(t *testing.T) {
	stt := &mockSTT{duration: 4.2}
	store := &mockVoiceQuotaStore{checkOK: false}
	logic := NewTranscribeLogic(context.Background(), transcribeSvcCtx(stt, store, 1800))

	_, err := logic.Transcribe(transcribeReq())
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
	assert.Equal(t, int32(0), stt.calls.Load(), "over-cap must not hit the provider")
	assert.Equal(t, int64(0), store.seconds.Load())
}

func TestTranscribe_QuotaStoreError_FailsClosed(t *testing.T) {
	stt := &mockSTT{duration: 4.2}
	store := &mockVoiceQuotaStore{checkErr: errors.New("redis down")}
	logic := NewTranscribeLogic(context.Background(), transcribeSvcCtx(stt, store, 1800))

	_, err := logic.Transcribe(transcribeReq())
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
	assert.Equal(t, int32(0), stt.calls.Load(), "store error must not reach the provider")
}

func TestTranscribe_CapConfiguredButNoStore_FailsClosed(t *testing.T) {
	stt := &mockSTT{duration: 4.2}
	logic := NewTranscribeLogic(context.Background(), transcribeSvcCtx(stt, nil, 1800))

	_, err := logic.Transcribe(transcribeReq())
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
	assert.Equal(t, int32(0), stt.calls.Load())
}

func TestTranscribe_STTError_NoUsageRecorded(t *testing.T) {
	stt := &mockSTT{err: errors.New("provider boom")}
	store := &mockVoiceQuotaStore{checkOK: true}
	logic := NewTranscribeLogic(context.Background(), transcribeSvcCtx(stt, store, 1800))

	_, err := logic.Transcribe(transcribeReq())
	require.Error(t, err)
	assert.Equal(t, int64(0), store.seconds.Load(), "failed transcription bills no seconds")
}
