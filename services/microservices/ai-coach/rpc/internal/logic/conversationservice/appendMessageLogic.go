package conversationservicelogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AppendMessageLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAppendMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppendMessageLogic {
	return &AppendMessageLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AppendMessageLogic) AppendMessage(in *aicoach.AppendMessageRequest) (*aicoach.AppendMessageResponse, error) {
	if l.svcCtx.Queries == nil {
		return nil, status.Error(codes.Unavailable, "conversation persistence is not configured")
	}
	if in.ConversationId == "" || in.UserId == "" || in.Content == "" {
		return nil, status.Error(codes.InvalidArgument, "conversationId, userId, and content are required")
	}

	role := in.Role
	if role != "user" && role != "assistant" {
		role = "user"
	}

	convID, err := parseUUID(in.ConversationId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid conversationId")
	}
	userID, err := parseUUID(in.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid userId")
	}

	// Verify the conversation belongs to the user.
	if _, err := l.svcCtx.Queries.GetConversation(l.ctx, convID, userID); err != nil {
		return nil, status.Error(codes.NotFound, "conversation not found")
	}

	msg, err := l.svcCtx.Queries.CreateMessage(l.ctx, convID, role, in.Content)
	if err != nil {
		l.Errorf("failed to append message: %v", err)
		return nil, status.Error(codes.Internal, "failed to append message")
	}

	// Update the conversation's last_message and updated_at.
	conv, err := l.svcCtx.Queries.UpdateConversationLastMessage(l.ctx, convID, in.Content)
	if err != nil {
		l.Errorf("failed to update conversation last_message: %v", err)
		// Still return the message; the conversation update is non-critical.
		conv, _ = l.svcCtx.Queries.GetConversation(l.ctx, convID, userID)
	}

	return &aicoach.AppendMessageResponse{
		Message:      protoMessage(msg),
		Conversation: protoConversation(conv),
	}, nil
}
