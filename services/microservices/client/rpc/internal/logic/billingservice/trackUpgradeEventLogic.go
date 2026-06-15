package billingservicelogic

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TrackUpgradeEventLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTrackUpgradeEventLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TrackUpgradeEventLogic {
	return &TrackUpgradeEventLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TrackUpgradeEventLogic) TrackUpgradeEvent(in *client.TrackUpgradeEventRequest) (*client.TrackUpgradeEventResponse, error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	metadata := json.RawMessage("{}")
	if in.MetadataJson != "" {
		metadata = json.RawMessage(in.MetadataJson)
	}

	var trigger *string
	if in.Trigger != "" {
		trigger = &in.Trigger
	}
	var planCode *string
	if in.PlanCode != "" {
		planCode = &in.PlanCode
	}
	var billingInterval *string
	if in.BillingInterval != "" {
		bi := (in.BillingInterval)
		billingInterval = &bi
	}
	var feedbackReason *string
	if in.FeedbackReason != "" {
		feedbackReason = &in.FeedbackReason
	}
	var feedbackNote *string
	if in.FeedbackNote != "" {
		feedbackNote = &in.FeedbackNote
	}

	event, err := l.svcCtx.Repo.Billing.CreateUpgradeEvent(l.ctx, db.CreateUpgradeEventParams{
		UserID:          userID,
		EventType:       (in.EventType),
		Surface:         in.Surface,
		TriggerSource:         trigger,
		Code:            planCodeValue(planCode),
		BillingInterval: billingInterval,
		FeedbackReason:  feedbackReason,
		FeedbackNote:    feedbackNote,
		Metadata:        metadata,
	})
	if err != nil {
		l.Errorf("Failed to create upgrade event: %v", err)
		return nil, status.Error(codes.Internal, "failed to track event")
	}

	return &client.TrackUpgradeEventResponse{
		EventId: event.ID.String(),
	}, nil
}

func planCodeValue(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
