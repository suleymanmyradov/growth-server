// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package auth

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	authservice "github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type VerifyEmailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewVerifyEmailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VerifyEmailLogic {
	return &VerifyEmailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *VerifyEmailLogic) VerifyEmail(req *types.VerifyEmailRequest) (*types.AuthResponse, error) {
	if req == nil || req.Token == "" {
		return nil, errInvalidArgument(MsgTokenRequired)
	}

	rpcResp, err := l.svcCtx.AuthRpc.VerifyEmail(l.ctx, &authservice.VerifyEmailRequest{
		Token: req.Token,
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
