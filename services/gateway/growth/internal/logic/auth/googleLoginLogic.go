// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package auth

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	authservice "github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
)

type GoogleLoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGoogleLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GoogleLoginLogic {
	return &GoogleLoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GoogleLoginLogic) GoogleLogin(req *types.GoogleLoginRequest) (*types.AuthResponse, error) {
	if req == nil || req.AuthorizationCode == "" {
		return nil, status.Error(codes.InvalidArgument, "authorization code is required")
	}

	rpcResp, err := l.svcCtx.AuthRpc.GoogleLogin(l.ctx, &authservice.GoogleLoginRequest{
		AuthorizationCode: req.AuthorizationCode,
		RedirectUri:       req.RedirectUri,
	})
	if err != nil {
		return nil, err
	}

	resp := &types.AuthResponse{
		AccessToken:  rpcResp.GetAccessToken(),
		RefreshToken: rpcResp.GetRefreshToken(),
		ExpiresIn:    rpcResp.GetExpiresIn(),
	}
	if rpcResp.GetUser() != nil {
		resp.User = mapAuthUserToProfile(rpcResp.GetUser())
	}
	return resp, nil
}
