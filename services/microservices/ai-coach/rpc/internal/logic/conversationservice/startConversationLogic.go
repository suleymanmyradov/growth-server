package conversationservicelogic

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/repository/db"
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

	// Create the conversation, the optional initial message, and the
	// last_message denormalization atomically — a swallowed failure here used
	// to leave a conversation with a stale/missing last_message.
	var conv db.Conversation
	var initialMsg db.ConversationMessage
	err = l.svcCtx.TxRunner.Run(l.ctx, in.UserId, func(tx pgx.Tx) error {
		qtx := l.svcCtx.Queries.WithTx(tx)

		var cErr error
		conv, cErr = qtx.CreateConversation(l.ctx, userID, title, convType)
		if cErr != nil {
			return fmt.Errorf("create conversation: %w", cErr)
		}

		if in.InitialMessage != "" {
			var mErr error
			initialMsg, mErr = qtx.CreateMessage(l.ctx, conv.ID, "user", in.InitialMessage)
			if mErr != nil {
				return fmt.Errorf("create initial message: %w", mErr)
			}
			conv, mErr = qtx.UpdateConversationLastMessage(l.ctx, conv.ID, in.InitialMessage)
			if mErr != nil {
				return fmt.Errorf("update conversation last_message: %w", mErr)
			}
		}
		return nil
	})
	if err != nil {
		l.Errorf("failed to start conversation: %v", err)
		return nil, status.Error(codes.Internal, "failed to create conversation")
	}

	resp := &aicoach.StartConversationResponse{
		Conversation: protoConversation(conv),
	}
	if in.InitialMessage != "" {
		resp.InitialMessageRow = protoMessage(initialMsg)
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
