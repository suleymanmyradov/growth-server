package conversationservicelogic

import (
	"context"
	"strings"

	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type StartConversationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewStartConversationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StartConversationLogic {
	return &StartConversationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *StartConversationLogic) StartConversation(in *aicoach.StartConversationRequest) (*aicoach.StartConversationResponse, error) {
	if l.svcCtx.Queries == nil {
		return nil, status.Error(codes.Unavailable, "conversation persistence is not configured")
	}
	if in.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "userId is required")
	}

	convType := in.Type
	if convType == "" {
		convType = "coach"
	}

	title := in.Title
	if title == "" && in.InitialMessage != "" {
		// Auto-title from first message.
		title = strings.TrimSpace(in.InitialMessage)
	}
	// Always clamp the title to fit the conversations.title varchar(255)
	// column. The frontend sends the full user message as the title for new
	// conversations, so without this guard long opening messages exceed the
	// column limit and the whole StartConversation fails with a 500.
	title = truncateTitle(title, 252)

	userID, err := parseUUID(in.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid userId")
	}

	conv, err := l.svcCtx.Queries.CreateConversation(l.ctx, userID, title, convType)
	if err != nil {
		l.Errorf("failed to create conversation: %v", err)
		return nil, status.Error(codes.Internal, "failed to create conversation")
	}

	resp := &aicoach.StartConversationResponse{
		Conversation: protoConversation(conv),
	}

	// If an initial message was provided, persist it and update the conversation's
	// last_message.
	if in.InitialMessage != "" {
		msg, err := l.svcCtx.Queries.CreateMessage(l.ctx, conv.ID, "user", in.InitialMessage)
		if err != nil {
			l.Errorf("failed to create initial message: %v", err)
			return nil, status.Error(codes.Internal, "failed to create initial message")
		}
		resp.InitialMessageRow = protoMessage(msg)

		updatedConv, err := l.svcCtx.Queries.UpdateConversationLastMessage(l.ctx, conv.ID, in.InitialMessage)
		if err != nil {
			l.Errorf("failed to update conversation last_message: %v", err)
		} else {
			resp.Conversation = protoConversation(updatedConv)
		}
	}

	l.Infof("started conversation: user=%s conv=%s", in.UserId, conv.ID)
	return resp, nil
}

// truncateTitle clamps a conversation title to maxLen characters (counting
// the trailing "..." when truncated). It trims surrounding whitespace first
// and collapses internal newlines/whitespace so multi-line opening messages
// produce a clean single-line title.
func truncateTitle(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	// Collapse newlines and runs of whitespace into single spaces.
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
