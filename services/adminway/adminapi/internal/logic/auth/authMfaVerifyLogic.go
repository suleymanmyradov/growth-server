// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package auth

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/mfa"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// maxMfaAttempts bounds code guessing against a single ticket before it is
// destroyed — on top of the per-IP auth rate limit the route already gets.
const maxMfaAttempts = 5

type AuthMfaVerifyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAuthMfaVerifyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthMfaVerifyLogic {
	return &AuthMfaVerifyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AuthMfaVerify completes login for an enrolled admin: it exchanges a
// verify-purpose ticket plus a TOTP code (or one-time backup code) for the
// real token pair. The ticket is single-use and destroyed on success or when
// its attempt budget runs out.
func (l *AuthMfaVerifyLogic) AuthMfaVerify(req *types.MfaVerifyRequest) (*types.AuthResponse, error) {
	if req.Ticket == "" || strings.TrimSpace(req.Code) == "" {
		return nil, errInvalidArgument(MsgMfaTicketAndCodeRequired)
	}

	ticket, err := l.svcCtx.Repo.Mfa.GetTicketByHash(l.ctx, mfa.HashTicket(req.Ticket))
	if err != nil || ticket.Purpose != mfa.TicketPurposeVerify || time.Now().After(ticket.ExpiresAt.Time) {
		return nil, ErrInvalidMfaTicket
	}
	if ticket.Attempts >= maxMfaAttempts {
		_ = l.svcCtx.Repo.Mfa.DeleteTicket(l.ctx, ticket.ID)
		return nil, ErrInvalidMfaTicket
	}

	user, err := l.svcCtx.Repo.InternalUsers.GetByID(l.ctx, ticket.UserID)
	if err != nil || !user.TotpEnabledAt.Valid || user.TotpSecretEncrypted == nil {
		// MFA was disabled (or the user deleted) between ticket issue and now.
		_ = l.svcCtx.Repo.Mfa.DeleteTicket(l.ctx, ticket.ID)
		return nil, ErrInvalidMfaTicket
	}

	if err := l.checkCode(user.TotpSecretEncrypted, ticket.UserID, req.Code); err != nil {
		_ = l.svcCtx.Repo.Mfa.IncrementTicketAttempts(l.ctx, ticket.ID)
		return nil, err
	}

	_ = l.svcCtx.Repo.Mfa.DeleteTicket(l.ctx, ticket.ID)

	auth, err := issueAuthTokens(l.ctx, l.svcCtx, user)
	if err != nil {
		l.Errorf("mfa verify failed to create tokens for user %s: %v", user.ID, err)
		return nil, ErrFailedGenAccessToken
	}
	return auth, nil
}

// checkCode accepts either a 6-digit TOTP or a one-time backup code.
func (l *AuthMfaVerifyLogic) checkCode(encryptedSecret *string, userID uuid.UUID, code string) error {
	code = strings.TrimSpace(code)
	if isSixDigits(code) {
		if len(l.svcCtx.MfaKey) == 0 {
			return ErrMfaNotConfigured
		}
		secret, err := mfa.DecryptSecret(l.svcCtx.MfaKey, *encryptedSecret)
		if err != nil {
			l.Errorf("mfa secret decrypt failed: %v", err)
			return errInternal(MsgInvalidMfaCode)
		}
		if mfa.ValidateCode(secret, code) {
			return nil
		}
		return ErrInvalidMfaCode
	}

	ok, err := l.svcCtx.Repo.Mfa.ConsumeBackupCode(l.ctx, userID, mfa.HashBackupCode(code))
	if err != nil {
		l.Errorf("backup code consume failed for user %s: %v", userID, err)
		return ErrInvalidMfaCode
	}
	if !ok {
		return ErrInvalidMfaCode
	}
	return nil
}

func isSixDigits(s string) bool {
	if len(s) != 6 {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
