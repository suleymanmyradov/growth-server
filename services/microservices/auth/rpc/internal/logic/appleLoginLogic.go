package logic

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/suleymanmyradov/growth-server/pkg/oauth/apple"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

const appleProvider = "apple"

type AppleLoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
	// testTxRunner is only set in tests to inject a no-op transaction runner.
	testTxRunner svc.TxRunnerInterface
}

func NewAppleLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppleLoginLogic {
	return &AppleLoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AppleLogin verifies the Apple identity token and either logs in an existing
// linked user, links an Apple identity to an existing user with the same email,
// or creates a new OAuth-only user. The user's name (only present on first
// authorization) is taken from the request's AppleFullName, not the ID token.
//
// If an authorization code is supplied and the Apple private key is configured,
// it is exchanged for a refresh token on a best-effort basis — failure is logged
// and does not fail the login, because the ID token is sufficient to identify
// the user.
func (l *AppleLoginLogic) AppleLogin(in *auth.AppleLoginRequest) (*auth.AuthResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "AppleLoginLogic.AppleLogin")
	defer span.End()

	if in == nil || in.IdentityToken == "" {
		return nil, errInvalidArgument(MsgIdentityTokenRequired)
	}

	cfg := l.svcCtx.Config.AppleOAuth
	if cfg.ServiceID == "" {
		l.Errorf("AppleLogin: Apple OAuth not configured (ServiceID missing)")
		return nil, errFailedPrecondition(MsgAppleNotConfigured)
	}

	// Validate a client-supplied redirect URI against the allowlist before any
	// code exchange. Mirrors the Google flow's allowlist check.
	if in.RedirectUri != "" {
		if !apple.IsAllowedRedirectURI(in.RedirectUri, cfg.AllowedRedirectURIs, cfg.RedirectURI) {
			l.Errorf("AppleLogin: redirect URI not allowed: %s", in.RedirectUri)
			return nil, errInvalidArgument(MsgRedirectURINotAllowed)
		}
	}

	verifier := apple.NewVerifier(apple.Config{
		ServiceID:           cfg.ServiceID,
		TeamID:              cfg.TeamID,
		KeyID:               cfg.KeyID,
		PrivateKeyPath:      cfg.PrivateKeyPath,
		PrivateKey:          cfg.PrivateKey,
		RedirectURI:         cfg.RedirectURI,
		AllowedRedirectURIs: cfg.AllowedRedirectURIs,
	}, nil)

	appleUser, err := verifier.VerifyIDToken(ctx, in.IdentityToken, in.Nonce)
	if err != nil {
		l.Errorf("AppleLogin: id token verification failed: %v", err)
		return nil, errUnauthenticated(MsgFailedAuthApple)
	}
	if appleUser.Subject == "" || appleUser.Email == "" {
		l.Errorf("AppleLogin: incomplete Apple profile (sub/email missing)")
		return nil, errUnauthenticated(MsgIncompleteAppleProfile)
	}

	// Apple only provides the name on the FIRST authorization, delivered in the
	// request body (not the ID token). Build the display name from AppleFullName.
	givenName, familyName := "", ""
	if in.FullName != nil {
		givenName = in.FullName.GivenName
		familyName = in.FullName.FamilyName
	}
	displayName := apple.FullName(givenName, familyName)

	var user db.User
	err = runInTx(l.svcCtx, l.testTxRunner, ctx, "", func(repo *repository.Repository) error {
		// 1. Already linked?
		acc, err := repo.Oauth.GetOAuthAccount(ctx, appleProvider, appleUser.Subject)
		if err == nil {
			row, gerr := repo.Users.GetUserByID(ctx, acc.UserID)
			if gerr != nil {
				return errInternal(MsgFailedLoadLinkedUser)
			}
			user = db.User(row)
			return enqueueUserProfileUpdated(ctx, repo.EventOutbox, user)
		}

		// 2. Existing user with the same email? Link the Apple identity to it.
		// Only when Apple verified the email — private relay addresses count as
		// verified because Apple generates them per (Apple account, Services ID)
		// pair, so they can never collide with another user's address.
		row, err := repo.Users.GetUserByEmail(ctx, appleUser.Email)
		if err == nil {
			if !appleUser.EmailVerified && !appleUser.IsPrivateRelayEmail {
				l.Infof("AppleLogin: refusing email link to existing user, email_verified=false (email=%s)", appleUser.Email)
				return ErrOAuthEmailNotVerified
			}
			user = db.User(row)
			emailPtr := &appleUser.Email
			if _, lerr := repo.Oauth.CreateOAuthAccount(ctx, user.ID, appleProvider, appleUser.Subject, emailPtr); lerr != nil {
				var pgErr *pgconn.PgError
				if errors.As(lerr, &pgErr) && pgErr.Code == "23505" {
					// Race: another request linked it. Treat as already linked.
					return enqueueUserProfileUpdated(ctx, repo.EventOutbox, user)
				}
				l.Errorf("AppleLogin: link to existing user failed: %v", lerr)
				return errInternal(MsgFailedLinkAppleAccount)
			}
			return enqueueUserProfileUpdated(ctx, repo.EventOutbox, user)
		}

		// 3. No existing user — create a new OAuth-only user.
		username := deriveUsername(appleUser.Email, displayName)
		// Ensure username uniqueness with a numeric suffix if needed.
		for i := 0; ; i++ {
			candidate := username
			if i > 0 {
				candidate = trimUsername(username) + itoa(i)
			}
			oauthRow, cerr := repo.Users.CreateUserOAuth(ctx, candidate, appleUser.Email, displayName, appleUser.EmailVerified)
			if cerr != nil {
				var pgErr *pgconn.PgError
				if errors.As(cerr, &pgErr) && pgErr.Code == "23505" {
					// Username or email collision — try the next suffix.
					continue
				}
				l.Errorf("AppleLogin: create user failed: %v", cerr)
				return errInternal(MsgFailedCreateUser)
			}
			user = db.User(oauthRow)
			emailPtr := &appleUser.Email
			if _, lerr := repo.Oauth.CreateOAuthAccount(ctx, user.ID, appleProvider, appleUser.Subject, emailPtr); lerr != nil {
				l.Errorf("AppleLogin: create oauth account failed: %v", lerr)
				return errInternal(MsgFailedLinkAppleAccount)
			}
			return enqueueUserProfileUpdated(ctx, repo.EventOutbox, user)
		}
	})
	if err != nil {
		return nil, err
	}

	// Best-effort authorization code exchange for a refresh token. Failure is
	// non-fatal: the ID token is sufficient to identify the user. Code exchange
	// is only needed for server-side token revocation / subscription status.
	if in.AuthorizationCode != "" && cfg.TeamID != "" && cfg.KeyID != "" {
		if _, exErr := verifier.ExchangeCode(ctx, in.AuthorizationCode, in.RedirectUri); exErr != nil {
			l.Infof("AppleLogin: code exchange skipped/failed (non-fatal): %v", exErr)
		}
	}

	sessionID := uuid.New()
	accessToken, err := l.svcCtx.TokenMaker.CreateAccessToken(ctx, user.ID, user.Username, []string{"user"}, sessionID)
	if err != nil {
		l.Errorf("AppleLogin: access token failed: %v", err)
		return nil, ErrFailedGenAccessToken
	}

	refreshToken, err := l.svcCtx.TokenMaker.CreateRefreshToken(ctx, user.ID, user.Username, []string{"user"}, sessionID)
	if err != nil {
		l.Errorf("AppleLogin: refresh token failed: %v", err)
		return nil, ErrFailedGenRefreshTok
	}

	l.Infof("AppleLogin successful for user %s (relay_email=%v)", user.ID, appleUser.IsPrivateRelayEmail)

	return &auth.AuthResponse{
		AccessToken:  accessToken.Token,
		RefreshToken: refreshToken.Token,
		ExpiresIn:    int64(l.svcCtx.Config.JWT.AccessExpiryDuration.Seconds()),
		User:         toPbUser(user),
	}, nil
}
