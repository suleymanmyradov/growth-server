package personalizationservicelogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdatePlanAdjustmentSuggestionStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdatePlanAdjustmentSuggestionStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdatePlanAdjustmentSuggestionStatusLogic {
	return &UpdatePlanAdjustmentSuggestionStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdatePlanAdjustmentSuggestionStatusLogic) UpdatePlanAdjustmentSuggestionStatus(in *client.UpdatePlanAdjustmentSuggestionStatusRequest) (*client.UpdatePlanAdjustmentSuggestionStatusResponse, error) {
	suggestionID, err := uuid.Parse(in.SuggestionId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid suggestion ID")
	}

	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	// Validate status
	validStatuses := map[string]bool{
		"pending":   true,
		"accepted":  true,
		"dismissed": true,
		"applied":   true,
	}
	if !validStatuses[in.Status] {
		return nil, status.Error(codes.InvalidArgument, "invalid status")
	}

	// Update suggestion status
	suggestion, err := l.svcCtx.Repo.PlanAdjustmentSuggestions.UpdatePlanAdjustmentSuggestionStatus(l.ctx, db.UpdatePlanAdjustmentSuggestionStatusParams{
		ID:     suggestionID,
		UserID: userID,
		Status: db.PlanAdjustmentStatusType(in.Status),
	})
	if err != nil {
		l.Errorf("failed to update plan adjustment suggestion status: %v", err)
		return nil, status.Error(codes.Internal, "failed to update plan adjustment suggestion status")
	}

	return &client.UpdatePlanAdjustmentSuggestionStatusResponse{
		Suggestion: dbPlanAdjustmentSuggestionToProto(suggestion),
	}, nil
}
