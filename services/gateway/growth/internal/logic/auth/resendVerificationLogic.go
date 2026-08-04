// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package auth

import (
	"context"

	"github.com/suleymanmyradov/growth-server/pkg/validator"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	authservice "github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResendVerificationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewResendVerificationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResendVerificationLogic {
	return &ResendVerificationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ResendVerificationLogic) ResendVerification(req *types.ResendVerificationRequest) (*types.EmptyResponse, error) {
	if req == nil || !validator.IsValidEmail(req.Email) {
		return nil, ErrValidEmailRequired
	}

	_, err := l.svcCtx.AuthRpc.ResendVerification(l.ctx, &authservice.ResendVerificationRequest{
		Email: req.Email,
	})
	if err != nil {
		return nil, err
	}

	return &types.EmptyResponse{}, nil
}
