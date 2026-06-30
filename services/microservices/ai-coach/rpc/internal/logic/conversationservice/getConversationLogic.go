package conversationservicelogic

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetConversationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetConversationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetConversationLogic {
	return &GetConversationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetConversationLogic) GetConversation(in *aicoach.GetConversationRequest) (*aicoach.GetConversationResponse, error) {
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

	conv, err := l.svcCtx.Queries.GetConversation(l.ctx, convID, userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, status.Error(codes.NotFound, "conversation not found")
		}
		l.Errorf("failed to get conversation: %v", err)
		return nil, status.Error(codes.Internal, "failed to get conversation")
	}

	return &aicoach.GetConversationResponse{
		Conversation: protoConversation(conv),
	}, nil
}
