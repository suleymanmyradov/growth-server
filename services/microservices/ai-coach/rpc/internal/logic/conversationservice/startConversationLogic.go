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
		// Auto-title from first message (truncated).
		title = strings.TrimSpace(in.InitialMessage)
		if len(title) > 60 {
			title = title[:60] + "..."
		}
	}

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
