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

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LoginLogic) Login(req *types.LoginRequest) (resp *types.AuthResponse, err error) {
	if !validator.IsNotEmpty(req.Email) || !validator.IsValidEmail(req.Email) {
		return nil, status.Error(codes.InvalidArgument, "valid email is required")
	}
	if !validator.IsNotEmpty(req.Password) {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}

	rpcReq := &authservice.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
		DeviceId: req.DeviceId,
	}

	rpcResp, err := l.svcCtx.AuthRpc.Login(l.ctx, rpcReq)
	if err != nil {
		return nil, err
	}

	return &types.AuthResponse{
		AccessToken:  rpcResp.AccessToken,
		RefreshToken: rpcResp.RefreshToken,
		ExpiresIn:    rpcResp.ExpiresIn,
		User: types.Profile{
			Id:            rpcResp.User.Id,
			FullName:      rpcResp.User.FullName,
			Username:      rpcResp.User.Username,
			Email:         rpcResp.User.Email,
			Bio:           rpcResp.User.Bio,
			Location:      rpcResp.User.Location,
			Website:       rpcResp.User.Website,
			Interests:     rpcResp.User.Interests,
			AvatarUrl:     rpcResp.User.AvatarUrl,
			CreatedAt:     rpcResp.User.CreatedAt,
			UpdatedAt:     rpcResp.User.UpdatedAt,
			EmailVerified: rpcResp.User.EmailVerified,
		},
	}, nil
}
