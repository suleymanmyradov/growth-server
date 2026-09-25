package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/suleymanmyradov/growth-server/pkg/oauth/google"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

const googleProvider = "google"

type GoogleLoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGoogleLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GoogleLoginLogic {
	return &GoogleLoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GoogleLogin authenticates via Google and either logs in an existing linked
// user, links a Google identity to an existing user with the same email, or
// creates a new OAuth-only user.
//
// Two credential paths are accepted:
//   - id_token (native): the app performed the PKCE exchange client-side with
//     a public iOS/Android OAuth client; the token is verified by signature
//     against Google's JWKS with an audience allowlist.
//   - authorization_code (web): the code is exchanged server-side with the
//     web client's secret, as before.
func (l *GoogleLoginLogic) GoogleLogin(in *auth.GoogleLoginRequest) (*auth.AuthResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GoogleLoginLogic.GoogleLogin")
	defer span.End()

	if in == nil || (in.AuthorizationCode == "" && in.IdToken == "") {
		return nil, errInvalidArgument(MsgAuthorizationCodeRequired)
	}

	var googleUser google.UserInfo
	if in.IdToken != "" {
		ui, err := l.verifyIDToken(ctx, in.IdToken)
		if err != nil {
			l.Errorf("GoogleLogin: id token verification failed: %v", err)
			return nil, err
		}
		googleUser = ui
	} else {
		ui, err := l.exchangeAuthCode(ctx, in.AuthorizationCode, in.RedirectUri)
		if err != nil {
			l.Errorf("GoogleLogin: exchange failed: %v", err)
			return nil, err
		}
		googleUser = ui
	}
	if googleUser.Subject == "" || googleUser.Email == "" {
		l.Errorf("GoogleLogin: incomplete Google profile (sub/email missing)")
		return nil, errUnauthenticated(MsgIncompleteGoogleProfile)
	}

	var user db.User
	err := l.svcCtx.TxRunner.Run(ctx, "", func(tx pgx.Tx) error {
		q := db.New(tx)

		// 1. Already linked?
		acc, err := q.GetOAuthAccount(ctx, googleProvider, googleUser.Subject)
		if err == nil {
			row, gerr := q.GetUserByID(ctx, acc.UserID)
			if gerr != nil {
				return errInternal(MsgFailedLoadLinkedUser)
			}
			user = db.User(row)
			return nil
		}

		// 2. Existing user with the same email? Link the Google identity to it.
		row, err := q.GetUserByEmail(ctx, googleUser.Email)
		if err == nil {
			user = db.User(row)
			emailPtr := &googleUser.Email
			if _, lerr := q.CreateOAuthAccount(ctx, user.ID, googleProvider, googleUser.Subject, emailPtr); lerr != nil {
				var pgErr *pgconn.PgError
				if errors.As(lerr, &pgErr) && pgErr.Code == "23505" {
					// Race: another request linked it. Treat as already linked.
					return nil
				}
				l.Errorf("GoogleLogin: link to existing user failed: %v", lerr)
				return errInternal(MsgFailedLinkGoogleAccount)
			}
			return nil
		}

		// 3. No existing user — create a new OAuth-only user.
		username := deriveUsername(googleUser.Email, googleUser.Name)
		// Ensure username uniqueness with a numeric suffix if needed.
		for i := 0; ; i++ {
			candidate := username
			if i > 0 {
				candidate = trimUsername(username) + itoa(i)
			}
			oauthRow, cerr := q.CreateUserOAuth(ctx, candidate, googleUser.Email, googleUser.Name, googleUser.EmailVerified)
			if cerr != nil {
				var pgErr *pgconn.PgError
				if errors.As(cerr, &pgErr) && pgErr.Code == "23505" {
					// Username or email collision — try the next suffix.
					continue
				}
				l.Errorf("GoogleLogin: create user failed: %v", cerr)
				return errInternal(MsgFailedCreateUser)
			}
			user = db.User(oauthRow)
			emailPtr := &googleUser.Email
			if _, lerr := q.CreateOAuthAccount(ctx, user.ID, googleProvider, googleUser.Subject, emailPtr); lerr != nil {
				l.Errorf("GoogleLogin: create oauth account failed: %v", lerr)
				return errInternal(MsgFailedLinkGoogleAccount)
			}
			return nil
		}
	})
	if err != nil {
		return nil, err
	}

	// Publish on every login so downstream recipient projections stay current.
	publishUserProfileUpdated(ctx, l.svcCtx.EventsPub, user)

	sessionID := uuid.New()
	accessToken, err := l.svcCtx.TokenMaker.CreateAccessToken(ctx, user.ID, user.Username, []string{"user"}, sessionID)
	if err != nil {
		l.Errorf("GoogleLogin: access token failed: %v", err)
		return nil, ErrFailedGenAccessToken
	}

	refreshToken, err := l.svcCtx.TokenMaker.CreateRefreshToken(ctx, user.ID, user.Username, []string{"user"}, sessionID)
	if err != nil {
		l.Errorf("GoogleLogin: refresh token failed: %v", err)
		return nil, ErrFailedGenRefreshTok
	}

	l.Infof("GoogleLogin successful for user %s", user.ID)

	return &auth.AuthResponse{
		AccessToken:  accessToken.Token,
		RefreshToken: refreshToken.Token,
		ExpiresIn:    int64(l.svcCtx.Config.JWT.AccessExpiryDuration.Seconds()),
		User:         toPbUser(user),
	}, nil
}

// verifyIDToken handles the native path: the app exchanged its PKCE code
// client-side and sent the resulting Google ID token. It is verified by
// signature against Google's JWKS; the audience allowlist is the set of our
// OAuth client IDs (web + iOS + Android).
func (l *GoogleLoginLogic) verifyIDToken(ctx context.Context, idToken string) (google.UserInfo, error) {
	cfg := l.svcCtx.Config.GoogleOAuth
	audiences := []string{cfg.ClientID, cfg.IOSClientID, cfg.AndroidClientID}
	hasAudience := false
	for _, a := range audiences {
		if a != "" {
			hasAudience = true
			break
		}
	}
	if !hasAudience {
		l.Errorf("GoogleLogin: no Google client IDs configured for ID token audience validation")
		return google.UserInfo{}, errFailedPrecondition(MsgGoogleNotConfigured)
	}
	verifier := google.NewIDTokenVerifier(audiences, nil)
	ui, err := verifier.VerifyIDToken(ctx, idToken)
	if err != nil {
		return google.UserInfo{}, errUnauthenticated(MsgFailedAuthGoogle)
	}
	return ui, nil
}

// exchangeAuthCode handles the web path: the authorization code is redeemed
// server-side using the web client's secret.
func (l *GoogleLoginLogic) exchangeAuthCode(ctx context.Context, code, redirectURI string) (google.UserInfo, error) {
	cfg := google.Config{
		ClientID:     l.svcCtx.Config.GoogleOAuth.ClientID,
		ClientSecret: l.svcCtx.Config.GoogleOAuth.ClientSecret,
		RedirectURI:  l.svcCtx.Config.GoogleOAuth.RedirectURI,
	}
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		l.Errorf("GoogleLogin: Google OAuth not configured")
		return google.UserInfo{}, errFailedPrecondition(MsgGoogleNotConfigured)
	}

	if redirectURI == "" {
		redirectURI = cfg.RedirectURI
	} else {
		// Validate client-supplied redirect URI against an explicit allowlist
		// to prevent authorization code interception. If no allowlist is
		// configured, only the server's configured RedirectURI is accepted.
		if !isAllowedRedirectURI(redirectURI, l.svcCtx.Config.GoogleOAuth.AllowedRedirectURIs, cfg.RedirectURI) {
			l.Errorf("GoogleLogin: redirect URI not allowed: %s", redirectURI)
			return google.UserInfo{}, errInvalidArgument(MsgRedirectURINotAllowed)
		}
	}

	ui, err := cfg.ExchangeCode(ctx, code, redirectURI)
	if err != nil {
		return google.UserInfo{}, errUnauthenticated(MsgFailedAuthGoogle)
	}
	return ui, nil
}

// deriveUsername builds a lowercase username from the email local part, falling
// back to the display name. Result matches the users.username_format CHECK
// constraint (lowercase, starts with a letter, [a-z0-9_-]).
func deriveUsername(email, name string) string {
	local := email
	if at := strings.Index(email, "@"); at > 0 {
		local = email[:at]
	}
	base := sanitizeUsername(local)
	if base == "" {
		base = sanitizeUsername(name)
	}
	if base == "" {
		base = "user"
	}
	return base
}

func sanitizeUsername(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	started := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			started = true
		case (r >= '0' && r <= '9') || r == '_' || r == '-':
			if started {
				b.WriteRune(r)
			}
		default:
			if started {
				b.WriteRune('_')
			}
		}
	}
	return strings.Trim(b.String(), "_-")
}

func trimUsername(s string) string {
	const maxLen = 45
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// isAllowedRedirectURI checks whether a client-supplied redirect URI is in the
// allowlist. If the allowlist is empty, only the server's configured RedirectURI
// is accepted. The comparison is exact to prevent path traversal tricks.
func isAllowedRedirectURI(uri string, allowed []string, configured string) bool {
	for _, a := range allowed {
		if a == uri {
			return true
		}
	}
	return uri == configured
}
