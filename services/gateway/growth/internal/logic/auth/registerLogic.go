// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package auth

import (
	"context"

	"github.com/suleymanmyradov/growth-server/pkg/validator"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	authservice "github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RegisterLogic) Register(req *types.RegisterRequest) (*types.AuthResponse, error) {
	if !validator.IsNotEmpty(req.Username) {
		return nil, status.Error(codes.InvalidArgument, "username is required")
	}
	if !validator.IsNotEmpty(req.Email) || !validator.IsValidEmail(req.Email) {
		return nil, status.Error(codes.InvalidArgument, "valid email is required")
	}
	if !validator.IsStrongPassword(req.Password) {
		return nil, status.Error(codes.InvalidArgument, "password must be at least 8 characters with uppercase, lowercase, number and special character")
	}
	if !validator.IsNotEmpty(req.FullName) {
		return nil, status.Error(codes.InvalidArgument, "full name is required")
	}

	registerResp, err := l.svcCtx.AuthRpc.Register(l.ctx, &authservice.RegisterRequest{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
		FullName: req.FullName,
	})
	if err != nil {
		return nil, err
	}

	resp := &types.AuthResponse{
		AccessToken:  registerResp.GetAccessToken(),
		RefreshToken: registerResp.GetRefreshToken(),
		ExpiresIn:    registerResp.GetExpiresIn(),
	}

	if registerResp.GetUser() != nil {
		resp.User = mapAuthUserToProfile(registerResp.GetUser())
	}

	return resp, nil
}

func mapAuthUserToProfile(user *authservice.User) types.Profile {
	if user == nil {
		return types.Profile{}
	}

	return types.Profile{
		Id:        user.GetId(),
		FullName:  user.GetFullName(),
		Username:  user.GetUsername(),
		Email:     user.GetEmail(),
		Bio:       user.GetBio(),
		Location:  user.GetLocation(),
		Website:   user.GetWebsite(),
		Interests: user.GetInterests(),
		AvatarUrl: user.GetAvatarUrl(),
		CreatedAt: user.GetCreatedAt(),
		UpdatedAt: user.GetUpdatedAt(),
	}
}
