// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package auth

import (
	"context"
	"strings"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/mfa"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	"golang.org/x/crypto/bcrypt"

	"github.com/zeromicro/go-zero/core/logx"
)

type AuthMfaDisableLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAuthMfaDisableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthMfaDisableLogic {
	return &AuthMfaDisableLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AuthMfaDisable turns off TOTP for the caller. It requires the account
// password plus a current TOTP code or backup code, so a stolen session alone
// cannot strip the second factor. The route already requires a full admin JWT.
func (l *AuthMfaDisableLogic) AuthMfaDisable(req *types.MfaDisableRequest) (*types.EmptyResponse, error) {
	if req.Password == "" || strings.TrimSpace(req.Code) == "" {
		return nil, errInvalidArgument(MsgEmailAndPasswordRequired)
	}

	user, err := callerUser(l.ctx, l.svcCtx)
	if err != nil {
		l.Errorf("mfa disable failed to resolve caller: %v", err)
		return nil, err
	}
	if !user.TotpEnabledAt.Valid || user.TotpSecretEncrypted == nil {
		return nil, ErrMfaNotEnabled
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	code := strings.TrimSpace(req.Code)
	valid := false
	if isSixDigits(code) && len(l.svcCtx.MfaKey) > 0 {
		secret, err := mfa.DecryptSecret(l.svcCtx.MfaKey, *user.TotpSecretEncrypted)
		if err != nil {
			l.Errorf("mfa disable failed to decrypt secret for user %s: %v", user.ID, err)
			return nil, errInternal(MsgInvalidMfaCode)
		}
		valid = mfa.ValidateCode(secret, code)
	} else {
		valid, err = l.svcCtx.Repo.Mfa.ConsumeBackupCode(l.ctx, user.ID, mfa.HashBackupCode(code))
		if err != nil {
			l.Errorf("mfa disable failed to check backup code for user %s: %v", user.ID, err)
			return nil, errInternal(MsgInvalidMfaCode)
		}
	}
	if !valid {
		return nil, ErrInvalidMfaCode
	}

	if err := l.svcCtx.Repo.Mfa.DisableTotp(l.ctx, user.ID); err != nil {
		l.Errorf("mfa disable failed for user %s: %v", user.ID, err)
		return nil, errInternal(MsgInvalidMfaCode)
	}
	_ = l.svcCtx.Repo.Mfa.DeleteBackupCodesForUser(l.ctx, user.ID)
	_ = l.svcCtx.Repo.Mfa.DeleteTicketsForUser(l.ctx, user.ID)

	return &types.EmptyResponse{}, nil
}
