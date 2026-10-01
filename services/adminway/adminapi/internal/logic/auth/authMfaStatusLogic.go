// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package auth

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AuthMfaStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAuthMfaStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthMfaStatusLogic {
	return &AuthMfaStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AuthMfaStatus reports whether the caller has TOTP enabled — the settings UI
// uses it to render the enroll vs. disable affordance.
func (l *AuthMfaStatusLogic) AuthMfaStatus() (*types.MfaStatusResponse, error) {
	user, err := callerUser(l.ctx, l.svcCtx)
	if err != nil {
		l.Errorf("mfa status failed to resolve caller: %v", err)
		return nil, err
	}
	return &types.MfaStatusResponse{Enabled: user.TotpEnabledAt.Valid}, nil
}
