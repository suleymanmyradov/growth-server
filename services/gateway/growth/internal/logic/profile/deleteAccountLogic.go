// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package profile

import (
	"context"
	"fmt"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	authservice "github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteAccountLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteAccountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteAccountLogic {
	return &DeleteAccountLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteAccountLogic) DeleteAccount() (resp *types.EmptyResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, fmt.Errorf("unauthenticated")
	}

	_, err = l.svcCtx.AuthRpc.DeleteUser(l.ctx, &authservice.DeleteUserRequest{
		UserId: p.UserID,
	})
	if err != nil {
		return nil, err
	}

	return &types.EmptyResponse{}, nil
}
