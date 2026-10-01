// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package auth

import (
	"context"
	"strings"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/mfa"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/middleware"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AuthMfaConfirmLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAuthMfaConfirmLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthMfaConfirmLogic {
	return &AuthMfaConfirmLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AuthMfaConfirm proves the caller can produce TOTP codes from the pending
// secret written by mfa/setup, then enables MFA and issues fresh backup
// codes. When the caller authorized with an enroll ticket (forced enrollment
// at login), the ticket is consumed and a full token pair is returned — that
// completes the login: password was proven at /auth/login, TOTP was just
// proven here.
func (l *AuthMfaConfirmLogic) AuthMfaConfirm(req *types.MfaConfirmRequest) (*types.MfaConfirmResponse, error) {
	if strings.TrimSpace(req.Code) == "" {
		return nil, errInvalidArgument(MsgMfaCodeRequired)
	}
	if len(l.svcCtx.MfaKey) == 0 {
		return nil, ErrMfaNotConfigured
	}

	user, err := callerUser(l.ctx, l.svcCtx)
	if err != nil {
		l.Errorf("mfa confirm failed to resolve caller: %v", err)
		return nil, err
	}
	if user.TotpSecretEncrypted == nil {
		return nil, ErrMfaSetupRequired
	}
	if user.TotpEnabledAt.Valid {
		return nil, ErrMfaAlreadyEnabled
	}

	secret, err := mfa.DecryptSecret(l.svcCtx.MfaKey, *user.TotpSecretEncrypted)
	if err != nil {
		l.Errorf("mfa confirm failed to decrypt secret for user %s: %v", user.ID, err)
		return nil, errInternal(MsgMfaSetupRequired)
	}
	if !mfa.ValidateCode(secret, strings.TrimSpace(req.Code)) {
		return nil, ErrInvalidMfaCode
	}

	if err := l.svcCtx.Repo.Mfa.EnableTotp(l.ctx, user.ID); err != nil {
		l.Errorf("mfa confirm failed to enable totp for user %s: %v", user.ID, err)
		return nil, errInternal(MsgMfaSetupRequired)
	}

	codes, hashes, err := mfa.NewBackupCodes()
	if err != nil {
		l.Errorf("mfa confirm failed to generate backup codes for user %s: %v", user.ID, err)
		return nil, errInternal(MsgMfaSetupRequired)
	}
	_ = l.svcCtx.Repo.Mfa.DeleteBackupCodesForUser(l.ctx, user.ID)
	for _, h := range hashes {
		if err := l.svcCtx.Repo.Mfa.InsertBackupCode(l.ctx, user.ID, h); err != nil {
			l.Errorf("mfa confirm failed to store backup code for user %s: %v", user.ID, err)
			return nil, errInternal(MsgMfaSetupRequired)
		}
	}

	resp := &types.MfaConfirmResponse{BackupCodes: codes}

	// Enroll-ticket callers hold no session yet — consume the ticket and issue
	// the real token pair to finish the login in one step.
	if ticket, ok := middleware.MfaTicketFrom(l.ctx); ok {
		_ = l.svcCtx.Repo.Mfa.DeleteTicket(l.ctx, ticket.ID)
		auth, err := issueAuthTokens(l.ctx, l.svcCtx, user)
		if err != nil {
			l.Errorf("mfa confirm failed to create tokens for user %s: %v", user.ID, err)
			return nil, ErrFailedGenAccessToken
		}
		resp.Auth = auth
	}

	return resp, nil
}
