package conversationservicelogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListConversationsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListConversationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListConversationsLogic {
	return &ListConversationsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListConversationsLogic) ListConversations(in *aicoach.ListConversationsRequest) (*aicoach.ListConversationsResponse, error) {
	if l.svcCtx.Queries == nil {
		return nil, status.Error(codes.Unavailable, "conversation persistence is not configured")
	}
	if in.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "userId is required")
	}

	userID, err := parseUUID(in.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid userId")
	}

	limit := in.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	page := in.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	convs, err := l.svcCtx.Queries.ListConversations(l.ctx, userID, limit, offset)
	if err != nil {
		l.Errorf("failed to list conversations: %v", err)
		return nil, status.Error(codes.Internal, "failed to list conversations")
	}

	total, err := l.svcCtx.Queries.CountConversations(l.ctx, userID)
	if err != nil {
		l.Errorf("failed to count conversations: %v", err)
		return nil, status.Error(codes.Internal, "failed to count conversations")
	}

	pbConvs := make([]*aicoach.Conversation, len(convs))
	for i, c := range convs {
		pbConvs[i] = protoConversation(c)
	}

	return &aicoach.ListConversationsResponse{
		Conversations: pbConvs,
		Total:         int32(total),
	}, nil
}
