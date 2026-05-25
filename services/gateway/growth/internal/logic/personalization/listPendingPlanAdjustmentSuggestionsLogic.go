// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package personalization

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientpersonalization "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/personalizationservice"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListPendingPlanAdjustmentSuggestionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPendingPlanAdjustmentSuggestionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPendingPlanAdjustmentSuggestionsLogic {
	return &ListPendingPlanAdjustmentSuggestionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPendingPlanAdjustmentSuggestionsLogic) ListPendingPlanAdjustmentSuggestions() (resp *types.PlanAdjustmentSuggestionsResponse, err error) {
	principal, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, errors.New("unauthorized")
	}

	rpcResp, err := l.svcCtx.PersonalizationRpc.ListPendingPlanAdjustmentSuggestions(l.ctx, &clientpersonalization.ListPendingPlanAdjustmentSuggestionsRequest{
		UserId: principal.UserID,
		Limit:  20,
		Offset: 0,
	})
	if err != nil {
		return nil, err
	}

	suggestions := make([]types.PlanAdjustmentSuggestion, len(rpcResp.Suggestions))
	for i, suggestion := range rpcResp.Suggestions {
		// Parse metadata JSON
		var metadata map[string]string
		if suggestion.MetadataJson != "" {
			if err := json.Unmarshal([]byte(suggestion.MetadataJson), &metadata); err != nil {
				metadata = make(map[string]string)
			}
		} else {
			metadata = make(map[string]string)
		}

		suggestions[i] = types.PlanAdjustmentSuggestion{
			Id:             suggestion.Id,
			UserId:         suggestion.UserId,
			AdjustmentType: suggestion.AdjustmentType,
			GoalId:         suggestion.GoalId,
			HabitId:        suggestion.HabitId,
			Source:         suggestion.Source,
			Reason:         suggestion.Reason,
			Suggestion:     suggestion.Suggestion,
			Metadata:       metadata,
			Status:         suggestion.Status,
			CreatedAt:      formatTimestamp(suggestion.CreatedAt),
			UpdatedAt:      formatTimestamp(suggestion.UpdatedAt),
		}
	}

	return &types.PlanAdjustmentSuggestionsResponse{
		Data: suggestions,
	}, nil
}
