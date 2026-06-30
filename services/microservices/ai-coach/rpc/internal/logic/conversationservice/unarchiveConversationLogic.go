package conversationservicelogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UnarchiveConversationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUnarchiveConversationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnarchiveConversationLogic {
	return &UnarchiveConversationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UnarchiveConversationLogic) UnarchiveConversation(in *aicoach.ArchiveConversationRequest) (*aicoach.ArchiveConversationResponse, error) {
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

	conv, err := l.svcCtx.Queries.UnarchiveConversation(l.ctx, convID, userID)
	if err != nil {
		l.Errorf("failed to unarchive conversation: %v", err)
		return nil, status.Error(codes.Internal, "failed to unarchive conversation")
	}

	return &aicoach.ArchiveConversationResponse{
		Conversation: protoConversation(conv),
	}, nil
}
