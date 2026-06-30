package personalization

import (
	"context"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/aicoachservice"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GenerateOnboardingHabitsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGenerateOnboardingHabitsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GenerateOnboardingHabitsLogic {
	return &GenerateOnboardingHabitsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GenerateOnboardingHabitsLogic) GenerateOnboardingHabits(req *types.GenerateOnboardingHabitsRequest) (*types.GenerateOnboardingHabitsResponse, error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	style := req.AccountabilityStyle
	if style == "" {
		style = "balanced"
	}

	rpcResp, err := l.svcCtx.AICoachRpc.GenerateOnboardingHabits(l.ctx, &aicoachservice.GenerateOnboardingHabitsRequest{
		UserId:              p.UserID,
		GoalTitle:           req.GoalTitle,
		GoalCategory:        req.GoalCategory,
		Motivation:          req.Motivation,
		Blocker:             req.Blocker,
		DailyMinutes:        req.DailyMinutes,
		AccountabilityStyle: style,
	})
	if err != nil {
		return nil, err
	}

	habits := make([]types.OnboardingHabitSuggestion, 0, len(rpcResp.Habits))
	for _, h := range rpcResp.Habits {
		habits = append(habits, types.OnboardingHabitSuggestion{
			Name:        h.Name,
			Description: h.Description,
		})
	}

	return &types.GenerateOnboardingHabitsResponse{
		Data: habits,
	}, nil
}
