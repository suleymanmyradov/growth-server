package logic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
	"github.com/zeromicro/go-zero/core/logx"
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
	if in == nil || in.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user ID is required")
	}

	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Errorf("failed to parse user ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	user, err := l.svcCtx.Repo.Users.GetUserByID(l.ctx, userID)
	if err != nil {
		l.Errorf("failed to get user: %v", err)
		return nil, status.Error(codes.NotFound, "user not found")
	}

	profile, err := l.svcCtx.Repo.Profiles.GetProfileByUserID(l.ctx, user.ID)
	if err != nil {
		l.Errorf("failed to get profile: %v", err)
		return nil, status.Error(codes.Internal, "failed to get profile")
	}

	return &auth.GetProfileResponse{
		User: toPbUser(user, profile),
	}, nil
}
