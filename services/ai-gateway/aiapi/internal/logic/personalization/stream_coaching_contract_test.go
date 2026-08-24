package personalization

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/config"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/types"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/conversationservice"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"
	"google.golang.org/grpc"
)

func TestParseProposalResult_ValidatesAndNormalizesPayload(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:  "valid proposal",
			input: `{"id":"proposal-1","action":"create_goal","payload":{"title":"Read"}}`,
		},
		{name: "invalid json", input: `{`, wantErr: true},
		{name: "missing id", input: `{"action":"create_goal","payload":{}}`, wantErr: true},
		{name: "missing action", input: `{"id":"proposal-1","payload":{}}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseProposalResult(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				require.Empty(t, got)
				return
			}
			require.NoError(t, err)
			require.JSONEq(t, tt.input, got)
		})
	}
}

func TestToAIAttachments_DropsEmptyDataAndPreservesFields(t *testing.T) {
	got := toAiAttachments([]types.Attachment{
		{AttachmentType: "image", Name: "plan.png", ContentType: "image/png", Data: "aW1hZ2U="},
		{AttachmentType: "document", Name: "empty.pdf", ContentType: "application/pdf"},
		{AttachmentType: "document", Name: "notes.txt", ContentType: "text/plain", Data: "dGV4dA=="},
	})

	require.Len(t, got, 2)
	require.Equal(t, "image", got[0].Type)
	require.Equal(t, "plan.png", got[0].Name)
	require.Equal(t, "image/png", got[0].ContentType)
	require.Equal(t, "aW1hZ2U=", got[0].Data)
	require.Equal(t, "document", got[1].Type)
	require.Equal(t, "notes.txt", got[1].Name)
}

type orderingConversationStore struct {
	events []string
}

func (s *orderingConversationStore) GetMessages(_ context.Context, _ *conversationservice.GetMessagesRequest, _ ...grpc.CallOption) (*conversationservice.GetMessagesResponse, error) {
	s.events = append(s.events, "get")
	return &conversationservice.GetMessagesResponse{}, nil
}

func (s *orderingConversationStore) AppendMessage(_ context.Context, in *conversationservice.AppendMessageRequest, _ ...grpc.CallOption) (*conversationservice.AppendMessageResponse, error) {
	s.events = append(s.events, "append:"+in.Role)
	return &conversationservice.AppendMessageResponse{}, nil
}

func (s *orderingConversationStore) RegenerateLastResponse(_ context.Context, _ *conversationservice.RegenerateLastResponseRequest, _ ...grpc.CallOption) (*conversationservice.RegenerateLastResponseResponse, error) {
	s.events = append(s.events, "regenerate")
	return &conversationservice.RegenerateLastResponseResponse{}, nil
}

func TestFetchAndPersistHistory_FetchesBeforeAppendingCurrentTurn(t *testing.T) {
	store := &orderingConversationStore{}
	history, err := fetchAndPersistHistory(context.Background(), &types.GeneratePersonalizedCoachingRequest{
		ConversationId: "conversation-1",
		UserMessage:    "current turn",
	}, testPrincipal(), store, config.CoachingConfig{})

	require.NoError(t, err)
	require.Empty(t, history)
	require.Equal(t, []string{"get", "append:user"}, store.events)
}

// failingAppendStore persists nothing: every append fails.
type failingAppendStore struct {
	orderingConversationStore
}

func (s *failingAppendStore) AppendMessage(_ context.Context, in *conversationservice.AppendMessageRequest, _ ...grpc.CallOption) (*conversationservice.AppendMessageResponse, error) {
	s.events = append(s.events, "append-failed:"+in.Role)
	return nil, errors.New("conversation store unavailable")
}

// A user turn that cannot be persisted must surface as an error, not be logged
// and swallowed. Swallowing it produced a complete coaching answer for a turn
// that no longer existed on reload.
func TestFetchAndPersistHistory_UserMessagePersistenceFailureIsFatal(t *testing.T) {
	store := &failingAppendStore{}
	history, err := fetchAndPersistHistory(context.Background(), &types.GeneratePersonalizedCoachingRequest{
		ConversationId: "conversation-1",
		UserMessage:    "current turn",
	}, testPrincipal(), store, config.CoachingConfig{})

	require.Error(t, err)
	require.Nil(t, history)
	require.Contains(t, store.events, "append-failed:user")
}

// A history *fetch* failure is a degraded answer, not a lost turn, so it must
// stay non-fatal — the user message still gets persisted.
func TestFetchAndPersistHistory_FetchFailureIsNonFatal(t *testing.T) {
	store := &fetchFailingStore{}
	history, err := fetchAndPersistHistory(context.Background(), &types.GeneratePersonalizedCoachingRequest{
		ConversationId: "conversation-1",
		UserMessage:    "current turn",
	}, testPrincipal(), store, config.CoachingConfig{})

	require.NoError(t, err)
	require.Empty(t, history)
	require.Equal(t, []string{"get-failed", "append:user"}, store.events)
}

type fetchFailingStore struct {
	orderingConversationStore
}

func (s *fetchFailingStore) GetMessages(_ context.Context, _ *conversationservice.GetMessagesRequest, _ ...grpc.CallOption) (*conversationservice.GetMessagesResponse, error) {
	s.events = append(s.events, "get-failed")
	return nil, errors.New("history unavailable")
}

// The model-context window is bounded independently of the UI page size, by
// both turn count and total characters.
func TestFetchAndPersistHistory_BoundsModelContextWindow(t *testing.T) {
	t.Run("turn count is passed to the query", func(t *testing.T) {
		store := &limitCapturingStore{}
		_, err := fetchAndPersistHistory(context.Background(), &types.GeneratePersonalizedCoachingRequest{
			ConversationId: "conversation-1",
			UserMessage:    "current turn",
		}, testPrincipal(), store, config.CoachingConfig{HistoryTurns: 7})
		require.NoError(t, err)
		require.Equal(t, int32(7), store.limit)
	})

	t.Run("default applies when unset", func(t *testing.T) {
		store := &limitCapturingStore{}
		_, err := fetchAndPersistHistory(context.Background(), &types.GeneratePersonalizedCoachingRequest{
			ConversationId: "conversation-1",
			UserMessage:    "current turn",
		}, testPrincipal(), store, config.CoachingConfig{})
		require.NoError(t, err)
		require.Equal(t, int32(20), store.limit)
	})

	t.Run("character budget drops the oldest turns", func(t *testing.T) {
		store := &limitCapturingStore{messages: []*aicoach.ConversationMessage{
			{Role: "user", Content: strings.Repeat("a", 100)},
			{Role: "assistant", Content: strings.Repeat("b", 100)},
			{Role: "user", Content: strings.Repeat("c", 100)},
		}}
		history, err := fetchAndPersistHistory(context.Background(), &types.GeneratePersonalizedCoachingRequest{
			ConversationId: "conversation-1",
			UserMessage:    "current turn",
		}, testPrincipal(), store, config.CoachingConfig{HistoryMaxChars: 250})
		require.NoError(t, err)
		// 3x100 chars exceeds 250, so the oldest turn is dropped and the two
		// most recent are kept in chronological order.
		require.Len(t, history, 2)
		require.Equal(t, strings.Repeat("b", 100), history[0].Content)
		require.Equal(t, strings.Repeat("c", 100), history[1].Content)
	})
}

type limitCapturingStore struct {
	orderingConversationStore
	limit    int32
	messages []*aicoach.ConversationMessage
}

func (s *limitCapturingStore) GetMessages(_ context.Context, in *conversationservice.GetMessagesRequest, _ ...grpc.CallOption) (*conversationservice.GetMessagesResponse, error) {
	s.limit = in.Limit
	s.events = append(s.events, "get")
	return &conversationservice.GetMessagesResponse{Messages: s.messages}, nil
}
