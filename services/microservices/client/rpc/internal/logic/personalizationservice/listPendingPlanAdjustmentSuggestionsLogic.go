package personalizationservicelogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListPendingPlanAdjustmentSuggestionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListPendingPlanAdjustmentSuggestionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPendingPlanAdjustmentSuggestionsLogic {
	return &ListPendingPlanAdjustmentSuggestionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListPendingPlanAdjustmentSuggestionsLogic) ListPendingPlanAdjustmentSuggestions(in *client.ListPendingPlanAdjustmentSuggestionsRequest) (*client.ListPendingPlanAdjustmentSuggestionsResponse, error) {
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	// Set default limit and offset
	limit := in.Limit
	if limit == 0 {
		limit = 20
	}
	offset := in.Offset

	suggestions, err := l.svcCtx.Repo.PlanAdjustmentSuggestions.ListPendingPlanAdjustmentSuggestions(l.ctx, userID, limit, offset)
	if err != nil {
		l.Errorf("failed to list pending plan adjustment suggestions: %v", err)
		return nil, status.Error(codes.Internal, "failed to list pending plan adjustment suggestions")
	}

	// Get total count
	total, err := l.svcCtx.Repo.PlanAdjustmentSuggestions.CountPendingPlanAdjustmentSuggestions(l.ctx, userID)
	if err != nil {
		l.Infof("failed to get total count: %v", err)
		total = int64(len(suggestions))
	}

	// Convert to proto
	protoSuggestions := make([]*client.PlanAdjustmentSuggestion, len(suggestions))
	for i, suggestion := range suggestions {
		protoSuggestions[i] = dbPlanAdjustmentSuggestionToProto(suggestion)
	}

	return &client.ListPendingPlanAdjustmentSuggestionsResponse{
		Suggestions: protoSuggestions,
		Total:       int32(total),
	}, nil
}
