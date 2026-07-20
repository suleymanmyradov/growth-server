package billingservicelogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSubscriptionStatusesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSubscriptionStatusesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSubscriptionStatusesLogic {
	return &ListSubscriptionStatusesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Admin: list every user's subscription plan code + status. Used by adminway
// to segment broadcast notifications into premium / free audiences.
func (l *ListSubscriptionStatusesLogic) ListSubscriptionStatuses(_ *client.ListSubscriptionStatusesRequest) (*client.ListSubscriptionStatusesResponse, error) {
	rows, err := l.svcCtx.Repo.Billing.ListSubscriptionStatuses(l.ctx)
	if err != nil {
		return nil, err
	}

	statuses := make([]*client.SubscriptionStatus, 0, len(rows))
	for _, row := range rows {
		statuses = append(statuses, &client.SubscriptionStatus{
			UserId:   row.UserID.String(),
			PlanCode: row.PlanCode,
			Status:   row.Status,
		})
	}

	return &client.ListSubscriptionStatusesResponse{Statuses: statuses}, nil
}
