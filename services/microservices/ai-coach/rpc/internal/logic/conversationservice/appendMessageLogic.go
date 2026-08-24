package conversationservicelogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/repository/db"
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

	// Strict role validation: an unrecognized role is a caller bug, not
	// something to silently coerce into "user". Coercion would persist a turn
	// under the wrong speaker and make the conversation record wrong in a way
	// nothing downstream can detect.
	role := in.Role
	if role != "user" && role != "assistant" {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported role %q", in.Role)
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

	// A client-supplied idempotency key routes to the upsert variant so a
	// retried send returns the stored message instead of duplicating the turn.
	// Server-authored messages (assistant replies, crisis responses) pass no
	// key and take the plain insert.
	var msg db.ConversationMessage
	if in.ClientMessageId != "" {
		clientID := in.ClientMessageId
		msg, err = l.svcCtx.Queries.CreateMessageIdempotent(l.ctx, convID, role, in.Content, &clientID)
	} else {
		msg, err = l.svcCtx.Queries.CreateMessage(l.ctx, convID, role, in.Content)
	}
	if err != nil {
		l.Errorf("failed to append message: %v", err)
		return nil, status.Error(codes.Internal, "failed to append message")
	}

	// Update the conversation's last_message and updated_at. If this fails we
	// cannot report a coherent conversation state, so surface the error rather
	// than returning a zero-valued Conversation that renders as a real one with
	// an all-zeros id.
	conv, err := l.svcCtx.Queries.UpdateConversationLastMessage(l.ctx, convID, in.Content)
	if err != nil {
		l.Errorf("failed to update conversation last_message: %v", err)
		conv, err = l.svcCtx.Queries.GetConversation(l.ctx, convID, userID)
		if err != nil {
			l.Errorf("failed to re-read conversation after last_message update failed: %v", err)
			return nil, status.Error(codes.Internal, "failed to append message")
		}
	}

	return &aicoach.AppendMessageResponse{
		Message:      protoMessage(msg),
		Conversation: protoConversation(conv),
	}, nil
}
