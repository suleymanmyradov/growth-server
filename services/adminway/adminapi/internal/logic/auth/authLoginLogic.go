package auth

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/mfa"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"golang.org/x/crypto/bcrypt"
)

type AuthLoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAuthLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthLoginLogic {
	return &AuthLoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AuthLoginLogic) AuthLogin(req *types.LoginRequest) (*types.LoginResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "AuthLoginLogic.AuthLogin")
	defer span.End()

	if req.Email == "" || req.Password == "" {
		return nil, errInvalidArgument(MsgEmailAndPasswordRequired)
	}

	user, err := l.svcCtx.Repo.InternalUsers.GetByEmail(ctx, req.Email)
	if err != nil {
		l.Errorf("login failed to get internal user by email: %v", err)
		return nil, ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		l.Errorf("login password mismatch for user %s: %v", user.ID, err)
		return nil, ErrInvalidCredentials
	}

	// Password is proven. Admins with TOTP enrolled get a verify-purpose
	// ticket instead of tokens; when Mfa.Required is on, unenrolled admins
	// get an enroll-purpose ticket that only reaches the mfa setup endpoints.
	switch {
	case user.TotpEnabledAt.Valid:
		ticket, err := issueMfaTicket(ctx, l.svcCtx, user.ID, mfa.TicketPurposeVerify)
		if err != nil {
			l.Errorf("login failed to issue mfa ticket for user %s: %v", user.ID, err)
			return nil, errInternal(MsgFailedGenAccessToken)
		}
		return &types.LoginResponse{MfaRequired: true, MfaTicket: ticket}, nil
	case l.svcCtx.Config.Mfa.Required:
		ticket, err := issueMfaTicket(ctx, l.svcCtx, user.ID, mfa.TicketPurposeEnroll)
		if err != nil {
			l.Errorf("login failed to issue mfa enroll ticket for user %s: %v", user.ID, err)
			return nil, errInternal(MsgFailedGenAccessToken)
		}
		return &types.LoginResponse{MfaRequired: true, MfaSetupRequired: true, MfaTicket: ticket}, nil
	}

	auth, err := issueAuthTokens(ctx, l.svcCtx, user)
	if err != nil {
		l.Errorf("login failed to create tokens for user %s: %v", user.ID, err)
		return nil, ErrFailedGenAccessToken
	}
	return &types.LoginResponse{Auth: auth}, nil
}
