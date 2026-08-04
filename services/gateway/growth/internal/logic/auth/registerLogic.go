// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package auth

import (
	"context"

	"github.com/suleymanmyradov/growth-server/pkg/validator"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	authservice "github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"

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

func (l *RegisterLogic) Register(req *types.RegisterRequest) (*types.RegisterResponse, error) {
	if !validator.IsNotEmpty(req.Username) {
		return nil, errInvalidArgument(MsgUsernameRequired)
	}
	if !validator.IsValidUsername(req.Username) {
		return nil, errInvalidArgument(MsgUsernameFormat)
	}
	if !validator.IsNotEmpty(req.Email) || !validator.IsValidEmail(req.Email) {
		return nil, ErrValidEmailRequired
	}
	if !validator.IsStrongPassword(req.Password) {
		return nil, errInvalidArgument(MsgPasswordStrength)
	}
	if !validator.IsNotEmpty(req.FullName) {
		return nil, errInvalidArgument(MsgFullNameRequired)
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

	return &types.RegisterResponse{
		RequiresVerification: registerResp.GetRequiresVerification(),
		Message:              registerResp.GetMessage(),
	}, nil
}

func mapAuthUserToProfile(user *authservice.User) types.Profile {
	if user == nil {
		return types.Profile{}
	}

	return types.Profile{
		Id:            user.GetId(),
		FullName:      user.GetFullName(),
		Username:      user.GetUsername(),
		Email:         user.GetEmail(),
		Bio:           user.GetBio(),
		Location:      user.GetLocation(),
		Website:       user.GetWebsite(),
		Interests:     user.GetInterests(),
		AvatarUrl:     user.GetAvatarUrl(),
		CreatedAt:     user.GetCreatedAt(),
		UpdatedAt:     user.GetUpdatedAt(),
		EmailVerified: user.GetEmailVerified(),
	}
}
