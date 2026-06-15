package logic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetProfileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProfileLogic {
	return &GetProfileLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetProfileLogic) GetProfile(in *auth.GetProfileRequest) (*auth.GetProfileResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetProfileLogic.GetProfile")
	defer span.End()

	l.Infof("GetProfile attempt")

	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		l.Errorf("GetProfile missing principal")
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("GetProfile failed to parse user ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	user, err := l.svcCtx.Repo.Users.GetUserByID(ctx, userID)
	if err != nil {
		l.Errorf("GetProfile failed to get user %s: %v", userID, err)
		return nil, status.Error(codes.NotFound, "user not found")
	}

	l.Infof("GetProfile successful for user %s", userID)

	return &auth.GetProfileResponse{
		User: toPbUser(user),
	}, nil
}
