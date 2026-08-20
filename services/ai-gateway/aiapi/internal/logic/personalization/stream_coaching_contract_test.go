package personalization

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/types"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/conversationservice"
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

func TestFetchAndPersistHistory_FetchesBeforeAppendingCurrentTurn(t *testing.T) {
	store := &orderingConversationStore{}
	history := fetchAndPersistHistory(context.Background(), &types.GeneratePersonalizedCoachingRequest{
		ConversationId: "conversation-1",
		UserMessage:    "current turn",
	}, testPrincipal(), store)

	require.Empty(t, history)
	require.Equal(t, []string{"get", "append:user"}, store.events)
}
