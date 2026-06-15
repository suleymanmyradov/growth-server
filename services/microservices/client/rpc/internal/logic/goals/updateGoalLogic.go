package goalslogic

import (
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type UpdateGoalLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateGoalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateGoalLogic {
	return &UpdateGoalLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateGoalLogic) UpdateGoal(in *client.UpdateGoalRequest) (*client.UpdateGoalResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "UpdateGoalLogic.UpdateGoal")
	defer span.End()
	goalID, err := uuid.Parse(in.GoalId)
	if err != nil {
		l.Errorf("Invalid goal ID: %v", err)
return nil, status.Error(codes.Internal, "invalid goal id")
	}

	var desc *string
	if in.Description != "" {
		desc = &in.Description
	}
	var dueTime pgtype.Timestamptz
	if in.DueDate > 0 {
		dueTime = pgtype.Timestamptz{Time: time.Unix(in.DueDate, 0), Valid: true}
	}


	params := db.UpdateGoalParams{
		ID:          goalID,
		Title:       in.Title,
		Description: desc,
		Slug:        in.Category,
		DueDate:     dueTime,
	}

	goal, err := l.svcCtx.Repo.Goals.UpdateGoal(ctx, params)
	if err != nil {
		l.Errorf("Failed to update goal: %v", err)
return nil, status.Error(codes.Internal, "failed to update goal")
	}

	return &client.UpdateGoalResponse{
		Goal: goalToProto(goal),
	}, nil
}
