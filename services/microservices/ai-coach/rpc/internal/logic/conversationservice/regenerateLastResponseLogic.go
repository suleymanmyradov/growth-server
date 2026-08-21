package conversationservicelogic

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegenerateLastResponseLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRegenerateLastResponseLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegenerateLastResponseLogic {
	return &RegenerateLastResponseLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RegenerateLastResponseLogic) RegenerateLastResponse(in *aicoach.RegenerateLastResponseRequest) (*aicoach.RegenerateLastResponseResponse, error) {
	if l.svcCtx.Queries == nil {
		return nil, status.Error(codes.Unavailable, "conversation persistence is not configured")
	}
	if in.ConversationId == "" || in.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "conversationId and userId are required")
	}

	convID, err := parseUUID(in.ConversationId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid conversationId")
	}
	userID, err := parseUUID(in.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid userId")
	}
	if _, err := l.svcCtx.Queries.GetConversation(l.ctx, convID, userID); err != nil {
		return nil, status.Error(codes.NotFound, "conversation not found")
	}

	if _, err := l.svcCtx.Queries.RegenerateLastResponse(l.ctx, convID); errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.FailedPrecondition, "conversation does not end with an assistant response")
	} else if err != nil {
		l.Errorf("failed to prepare response regeneration: %v", err)
		return nil, status.Error(codes.Internal, "failed to regenerate response")
	}
	userMessage, err := l.svcCtx.Queries.GetLastMessage(l.ctx, convID)
	if err != nil || userMessage.Role != "user" {
		l.Errorf("failed to find user message after preparing regeneration: %v", err)
		return nil, status.Error(codes.FailedPrecondition, "assistant response has no preceding user message")
	}
	if _, err := l.svcCtx.Queries.UpdateConversationLastMessage(l.ctx, convID, userMessage.Content); err != nil {
		l.Errorf("failed to update conversation after preparing regeneration: %v", err)
	}

	return &aicoach.RegenerateLastResponseResponse{UserMessage: protoMessage(userMessage)}, nil
}
