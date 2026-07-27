package logic

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/suleymanmyradov/growth-server/pkg/oauth/apple"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const appleProvider = "apple"

type AppleLoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
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
		return nil, status.Error(codes.InvalidArgument, "identity token is required")
	}

	cfg := l.svcCtx.Config.AppleOAuth
	if cfg.ServiceID == "" {
		l.Errorf("AppleLogin: Apple OAuth not configured (ServiceID missing)")
		return nil, status.Error(codes.FailedPrecondition, "Apple sign-in is not configured")
	}

	// Validate a client-supplied redirect URI against the allowlist before any
	// code exchange. Mirrors the Google flow's allowlist check.
	if in.RedirectUri != "" {
		if !apple.IsAllowedRedirectURI(in.RedirectUri, cfg.AllowedRedirectURIs, cfg.RedirectURI) {
			l.Errorf("AppleLogin: redirect URI not allowed: %s", in.RedirectUri)
			return nil, status.Error(codes.InvalidArgument, "redirect URI not allowed")
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
		return nil, status.Error(codes.Unauthenticated, "failed to authenticate with Apple")
	}
	if appleUser.Subject == "" || appleUser.Email == "" {
		l.Errorf("AppleLogin: incomplete Apple profile (sub/email missing)")
		return nil, status.Error(codes.Unauthenticated, "incomplete Apple profile")
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
	var isNewUser bool
	err = l.svcCtx.TxRunner.Run(ctx, "", func(tx pgx.Tx) error {
		q := db.New(tx)

		// 1. Already linked?
		acc, err := q.GetOAuthAccount(ctx, appleProvider, appleUser.Subject)
		if err == nil {
			row, gerr := q.GetUserByID(ctx, acc.UserID)
			if gerr != nil {
				return status.Error(codes.Internal, "failed to load linked user")
			}
			user = db.User(row)
			return nil
		}

		// 2. Existing user with the same email? Link the Apple identity to it.
		row, err := q.GetUserByEmail(ctx, appleUser.Email)
		if err == nil {
			user = db.User(row)
			emailPtr := &appleUser.Email
			if _, lerr := q.CreateOAuthAccount(ctx, user.ID, appleProvider, appleUser.Subject, emailPtr); lerr != nil {
				var pgErr *pgconn.PgError
				if errors.As(lerr, &pgErr) && pgErr.Code == "23505" {
					// Race: another request linked it. Treat as already linked.
					return nil
				}
				l.Errorf("AppleLogin: link to existing user failed: %v", lerr)
				return status.Error(codes.Internal, "failed to link Apple account")
			}
			return nil
		}

		// 3. No existing user — create a new OAuth-only user.
		isNewUser = true
		username := deriveUsername(appleUser.Email, displayName)
		// Ensure username uniqueness with a numeric suffix if needed.
		for i := 0; ; i++ {
			candidate := username
			if i > 0 {
				candidate = trimUsername(username) + itoa(i)
			}
			oauthRow, cerr := q.CreateUserOAuth(ctx, candidate, appleUser.Email, displayName, appleUser.EmailVerified)
			if cerr != nil {
				var pgErr *pgconn.PgError
				if errors.As(cerr, &pgErr) && pgErr.Code == "23505" {
					// Username or email collision — try the next suffix.
					continue
				}
				l.Errorf("AppleLogin: create user failed: %v", cerr)
				return status.Error(codes.Internal, "failed to create user")
			}
			user = db.User(oauthRow)
			emailPtr := &appleUser.Email
			if _, lerr := q.CreateOAuthAccount(ctx, user.ID, appleProvider, appleUser.Subject, emailPtr); lerr != nil {
				l.Errorf("AppleLogin: create oauth account failed: %v", lerr)
				return status.Error(codes.Internal, "failed to link Apple account")
			}
			return nil
		}
	})
	if err != nil {
		return nil, err
	}

	// Only publish on new user creation; existing users don't change profile on login.
	if isNewUser {
		publishUserProfileUpdated(ctx, l.svcCtx.EventsPub, user)
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
		return nil, status.Error(codes.Internal, "failed to generate access token")
	}

	refreshToken, err := l.svcCtx.TokenMaker.CreateRefreshToken(ctx, user.ID, user.Username, []string{"user"}, sessionID)
	if err != nil {
		l.Errorf("AppleLogin: refresh token failed: %v", err)
		return nil, status.Error(codes.Internal, "failed to generate refresh token")
	}

	l.Infof("AppleLogin successful for user %s (relay_email=%v)", user.ID, appleUser.IsPrivateRelayEmail)

	return &auth.AuthResponse{
		AccessToken:  accessToken.Token,
		RefreshToken: refreshToken.Token,
		ExpiresIn:    int64(l.svcCtx.Config.JWT.AccessExpiryDuration.Seconds()),
		User:         toPbUser(user),
	}, nil
}
