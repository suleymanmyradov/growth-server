package conversationservicelogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetMessagesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMessagesLogic {
	return &GetMessagesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetMessagesLogic) GetMessages(in *aicoach.GetMessagesRequest) (*aicoach.GetMessagesResponse, error) {
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

	// Verify the conversation belongs to the user.
	if _, err := l.svcCtx.Queries.GetConversation(l.ctx, convID, userID); err != nil {
		return nil, status.Error(codes.NotFound, "conversation not found")
	}

	limit := in.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	page := in.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	msgs, err := l.svcCtx.Queries.ListMessages(l.ctx, convID, limit, offset)
	if err != nil {
		l.Errorf("failed to list messages: %v", err)
		return nil, status.Error(codes.Internal, "failed to list messages")
	}

	total, err := l.svcCtx.Queries.CountMessages(l.ctx, convID)
	if err != nil {
		l.Errorf("failed to count messages: %v", err)
		return nil, status.Error(codes.Internal, "failed to count messages")
	}

	pbMsgs := make([]*aicoach.ConversationMessage, len(msgs))
	for i, m := range msgs {
		pbMsgs[i] = protoMessage(m)
	}

	return &aicoach.GetMessagesResponse{
		Messages: pbMsgs,
		Total:    int32(total),
	}, nil
}
