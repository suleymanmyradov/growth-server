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

type AppleLoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAppleLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppleLoginLogic {
	return &AppleLoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AppleLoginLogic) AppleLogin(req *types.AppleLoginRequest) (*types.AuthResponse, error) {
	if req == nil || req.IdentityToken == "" {
		return nil, status.Error(codes.InvalidArgument, "identity token is required")
	}

	var fullName *authservice.AppleFullName
	if req.FullName != nil {
		fullName = &authservice.AppleFullName{
			GivenName:  req.FullName.GivenName,
			FamilyName: req.FullName.FamilyName,
		}
	}

	rpcResp, err := l.svcCtx.AuthRpc.AppleLogin(l.ctx, &authservice.AppleLoginRequest{
		AuthorizationCode: req.AuthorizationCode,
		IdentityToken:     req.IdentityToken,
		Nonce:             req.Nonce,
		FullName:          fullName,
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
