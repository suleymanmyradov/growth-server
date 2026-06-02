// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package personalization

import (
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	"context"
	"encoding/json"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientpersonalization "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/personalizationservice"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
)

type ApplyPlanAdjustmentSuggestionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewApplyPlanAdjustmentSuggestionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplyPlanAdjustmentSuggestionLogic {
	return &ApplyPlanAdjustmentSuggestionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApplyPlanAdjustmentSuggestionLogic) ApplyPlanAdjustmentSuggestion(req *types.ApplyPlanAdjustmentSuggestionRequest) (resp *types.PlanAdjustmentSuggestionResponse, err error) {
	principal, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	rpcResp, err := l.svcCtx.PersonalizationRpc.ApplyPlanAdjustmentSuggestion(l.ctx, &clientpersonalization.ApplyPlanAdjustmentSuggestionRequest{
		SuggestionId: req.Id,
		UserId:       principal.UserID,
	})
	if err != nil {
		return nil, err
	}

	// Parse metadata JSON
	var metadata map[string]string
	if rpcResp.Suggestion.MetadataJson != "" {
		if err := json.Unmarshal([]byte(rpcResp.Suggestion.MetadataJson), &metadata); err != nil {
			metadata = make(map[string]string)
		}
	} else {
		metadata = make(map[string]string)
	}

	return &types.PlanAdjustmentSuggestionResponse{
		Data: types.PlanAdjustmentSuggestion{
			Id:             rpcResp.Suggestion.Id,
			UserId:         rpcResp.Suggestion.UserId,
			AdjustmentType: rpcResp.Suggestion.AdjustmentType,
			GoalId:         rpcResp.Suggestion.GoalId,
			HabitId:        rpcResp.Suggestion.HabitId,
			Source:         rpcResp.Suggestion.Source,
			Reason:         rpcResp.Suggestion.Reason,
			Suggestion:     rpcResp.Suggestion.Suggestion,
			Metadata:       metadata,
			Status:         rpcResp.Suggestion.Status,
			CreatedAt:      formatTimestamp(rpcResp.Suggestion.CreatedAt),
			UpdatedAt:      formatTimestamp(rpcResp.Suggestion.UpdatedAt),
		},
	}, nil
}
