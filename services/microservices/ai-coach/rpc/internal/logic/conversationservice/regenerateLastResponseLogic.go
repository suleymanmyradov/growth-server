package conversationservicelogic

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/repository/db"
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

	// Delete the assistant tail, locate the preceding user message, and refresh
	// the conversation's last_message atomically — a partial regeneration used
	// to leave last_message pointing at a deleted assistant turn.
	var userMessage db.ConversationMessage
	err = l.svcCtx.TxRunner.Run(l.ctx, in.UserId, func(tx pgx.Tx) error {
		qtx := l.svcCtx.Queries.WithTx(tx)

		if _, err := qtx.RegenerateLastResponse(l.ctx, convID); errors.Is(err, pgx.ErrNoRows) {
			return status.Error(codes.FailedPrecondition, "conversation does not end with an assistant response")
		} else if err != nil {
			return fmt.Errorf("prepare response regeneration: %w", err)
		}
		var err error
		userMessage, err = qtx.GetLastMessage(l.ctx, convID)
		if err != nil || userMessage.Role != "user" {
			return status.Error(codes.FailedPrecondition, "assistant response has no preceding user message")
		}
		if _, err := qtx.UpdateConversationLastMessage(l.ctx, convID, userMessage.Content); err != nil {
			return fmt.Errorf("update conversation last_message: %w", err)
		}
		return nil
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() != codes.Unknown {
			return nil, st.Err()
		}
		l.Errorf("failed to regenerate response: %v", err)
		return nil, status.Error(codes.Internal, "failed to regenerate response")
	}

	return &aicoach.RegenerateLastResponseResponse{UserMessage: protoMessage(userMessage)}, nil
}
