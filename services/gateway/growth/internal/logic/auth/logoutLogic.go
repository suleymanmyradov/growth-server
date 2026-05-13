// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package auth

import (
	"context"
	"strings"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	authservice "github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type LogoutLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LogoutLogic) Logout() (resp *types.EmptyResponse, err error) {
	authHeader := l.ctx.Value("Authorization")
	if authHeader == nil {
		return &types.EmptyResponse{}, nil
	}

	authHeaderValue, ok := authHeader.(string)
	if !ok {
		return &types.EmptyResponse{}, nil
	}
	token := strings.TrimPrefix(authHeaderValue, "Bearer ")

	_, err = l.svcCtx.AuthRpc.Logout(l.ctx, &authservice.LogoutRequest{
		AccessToken: token,
	})
	if err != nil {
		return nil, err
	}

	return &types.EmptyResponse{}, nil
}
