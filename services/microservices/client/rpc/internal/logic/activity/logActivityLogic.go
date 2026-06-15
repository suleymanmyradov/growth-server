package activitylogic

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LogActivityLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLogActivityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogActivityLogic {
	return &LogActivityLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LogActivityLogic) LogActivity(in *client.LogActivityRequest) (*client.LogActivityResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "LogActivityLogic.LogActivity")
	defer span.End()
	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, status.Error(codes.Internal, "invalid user id")
	}

	var metadata json.RawMessage
	if len(in.Metadata) > 0 {
		if b, err := json.Marshal(in.Metadata); err == nil {
			metadata = b
		}
	}

	var description *string
	if in.Description != "" {
		description = &in.Description
	}

	activity, err := l.svcCtx.Repo.Activities.LogActivity(ctx, db.LogActivityParams{
		UserID:      userID,
		Type:        (in.Type),
		Title:       in.Description,
		Description: description,
		Metadata:    metadata,
	})
	if err != nil {
		l.Errorf("Failed to create activity: %v", err)
		return nil, status.Error(codes.Internal, "failed to create activity")
	}

	return &client.LogActivityResponse{
		ActivityId: activity.ID.String(),
	}, nil
}
