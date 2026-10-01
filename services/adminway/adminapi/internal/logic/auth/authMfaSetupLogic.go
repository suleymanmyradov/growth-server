// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package auth

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/mfa"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AuthMfaSetupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAuthMfaSetupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthMfaSetupLogic {
	return &AuthMfaSetupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AuthMfaSetup generates a pending TOTP secret for the caller and returns the
// secret plus a rendered QR. The secret is inert until mfa/confirm proves the
// caller can produce codes from it — an abandoned setup does not enable MFA.
func (l *AuthMfaSetupLogic) AuthMfaSetup() (*types.MfaSetupResponse, error) {
	if len(l.svcCtx.MfaKey) == 0 {
		return nil, ErrMfaNotConfigured
	}

	user, err := callerUser(l.ctx, l.svcCtx)
	if err != nil {
		l.Errorf("mfa setup failed to resolve caller: %v", err)
		return nil, err
	}
	if user.TotpEnabledAt.Valid {
		return nil, ErrMfaAlreadyEnabled
	}

	issuer := l.svcCtx.Config.Mfa.Issuer
	if issuer == "" {
		issuer = "Growth Admin"
	}
	key, err := mfa.NewTOTPKey(issuer, user.Email)
	if err != nil {
		l.Errorf("mfa setup failed to generate totp key for user %s: %v", user.ID, err)
		return nil, errInternal(MsgMfaSetupRequired)
	}
	encSecret, err := mfa.EncryptSecret(l.svcCtx.MfaKey, key.Secret())
	if err != nil {
		l.Errorf("mfa setup failed to encrypt secret for user %s: %v", user.ID, err)
		return nil, errInternal(MsgMfaSetupRequired)
	}
	if err := l.svcCtx.Repo.Mfa.SetTotpSecret(l.ctx, user.ID, encSecret); err != nil {
		l.Errorf("mfa setup failed to store secret for user %s: %v", user.ID, err)
		return nil, errInternal(MsgMfaSetupRequired)
	}
	qrDataURL, err := mfa.QRCodeDataURL(key)
	if err != nil {
		l.Errorf("mfa setup failed to render qr for user %s: %v", user.ID, err)
		return nil, errInternal(MsgMfaSetupRequired)
	}

	return &types.MfaSetupResponse{
		Secret:        key.Secret(),
		OtpauthUrl:    key.URL(),
		QrCodeDataUrl: qrDataURL,
	}, nil
}
