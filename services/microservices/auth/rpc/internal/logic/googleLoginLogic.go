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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

// GoogleLogin exchanges the OAuth authorization code for Google user info and
// either logs in an existing linked user, links a Google identity to an
// existing user with the same email, or creates a new OAuth-only user.
func (l *GoogleLoginLogic) GoogleLogin(in *auth.GoogleLoginRequest) (*auth.AuthResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GoogleLoginLogic.GoogleLogin")
	defer span.End()

	if in == nil || in.AuthorizationCode == "" {
		return nil, status.Error(codes.InvalidArgument, "authorization code is required")
	}

	cfg := google.Config{
		ClientID:     l.svcCtx.Config.GoogleOAuth.ClientID,
		ClientSecret: l.svcCtx.Config.GoogleOAuth.ClientSecret,
		RedirectURI:  l.svcCtx.Config.GoogleOAuth.RedirectURI,
	}
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		l.Errorf("GoogleLogin: Google OAuth not configured")
		return nil, status.Error(codes.FailedPrecondition, "Google sign-in is not configured")
	}

	redirectURI := in.RedirectUri
	if redirectURI == "" {
		redirectURI = cfg.RedirectURI
	}

	googleUser, err := cfg.ExchangeCode(ctx, in.AuthorizationCode, redirectURI)
	if err != nil {
		l.Errorf("GoogleLogin: exchange failed: %v", err)
		return nil, status.Error(codes.Unauthenticated, "failed to authenticate with Google")
	}
	if googleUser.Subject == "" || googleUser.Email == "" {
		l.Errorf("GoogleLogin: incomplete Google profile (sub/email missing)")
		return nil, status.Error(codes.Unauthenticated, "incomplete Google profile")
	}

	var user db.User
	err = l.svcCtx.TxRunner.Run(ctx, "", func(tx pgx.Tx) error {
		q := db.New(tx)
		oauthRepo := l.svcCtx.Repo.Oauth

		// 1. Already linked?
		acc, err := oauthRepo.GetOAuthAccount(ctx, googleProvider, googleUser.Subject)
		if err == nil {
			row, gerr := q.GetUserByID(ctx, acc.UserID)
			if gerr != nil {
				return status.Error(codes.Internal, "failed to load linked user")
			}
			user = db.User(row)
			return nil
		}

		// 2. Existing user with the same email? Link the Google identity to it.
		row, err := q.GetUserByEmail(ctx, googleUser.Email)
		if err == nil {
			user = db.User(row)
			emailPtr := &googleUser.Email
			if _, lerr := oauthRepo.CreateOAuthAccount(ctx, user.ID, googleProvider, googleUser.Subject, emailPtr); lerr != nil {
				var pgErr *pgconn.PgError
				if errors.As(lerr, &pgErr) && pgErr.Code == "23505" {
					// Race: another request linked it. Treat as already linked.
					return nil
				}
				l.Errorf("GoogleLogin: link to existing user failed: %v", lerr)
				return status.Error(codes.Internal, "failed to link Google account")
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
				return status.Error(codes.Internal, "failed to create user")
			}
			user = db.User(oauthRow)
			emailPtr := &googleUser.Email
			if _, lerr := oauthRepo.CreateOAuthAccount(ctx, user.ID, googleProvider, googleUser.Subject, emailPtr); lerr != nil {
				l.Errorf("GoogleLogin: create oauth account failed: %v", lerr)
				return status.Error(codes.Internal, "failed to link Google account")
			}
			return nil
		}
	})
	if err != nil {
		return nil, err
	}

	sessionID := uuid.New()
	accessToken, err := l.svcCtx.TokenMaker.CreateAccessToken(ctx, user.ID, user.Username, []string{"user"}, sessionID)
	if err != nil {
		l.Errorf("GoogleLogin: access token failed: %v", err)
		return nil, status.Error(codes.Internal, "failed to generate access token")
	}

	refreshToken, err := l.svcCtx.TokenMaker.CreateRefreshToken(ctx, user.ID, user.Username, []string{"user"}, sessionID)
	if err != nil {
		l.Errorf("GoogleLogin: refresh token failed: %v", err)
		return nil, status.Error(codes.Internal, "failed to generate refresh token")
	}

	l.Infof("GoogleLogin successful for user %s", user.ID)

	return &auth.AuthResponse{
		AccessToken:  accessToken.Token,
		RefreshToken: refreshToken.Token,
		ExpiresIn:    int64(l.svcCtx.Config.JWT.AccessExpiryDuration.Seconds()),
		User:         toPbUser(user),
	}, nil
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
