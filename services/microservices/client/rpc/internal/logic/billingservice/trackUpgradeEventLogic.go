package billingservicelogic

import (
	"context"
	"database/sql"
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

	event, err := l.svcCtx.Repo.Billing.CreateUpgradeEvent(l.ctx, db.CreateUpgradeEventParams{
		UserID:          userID,
		EventType:       db.UpgradeEventType(in.EventType),
		Surface:         in.Surface,
		Trigger:         stringToNullString(in.Trigger),
		PlanCode:        stringToNullString(in.PlanCode),
		BillingInterval: stringToNullBillingInterval(in.BillingInterval),
		FeedbackReason:  stringToNullString(in.FeedbackReason),
		FeedbackNote:    stringToNullString(in.FeedbackNote),
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

func stringToNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func stringToNullBillingInterval(s string) db.NullBillingIntervalType {
	if s == "" {
		return db.NullBillingIntervalType{}
	}
	return db.NullBillingIntervalType{BillingIntervalType: db.BillingIntervalType(s), Valid: true}
}
