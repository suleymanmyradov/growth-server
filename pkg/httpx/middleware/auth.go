package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	"github.com/zeromicro/go-zero/rest"
)

// AccessTokenVerifier is the minimal token-verification surface the
// middleware needs. *jwt.Verifier and *jwt.TokenMaker both satisfy it.
type AccessTokenVerifier interface {
	VerifyAccessToken(ctx context.Context, tokenString string) (*jwt.TokenClaims, error)
}

// JWTMiddleware returns a go-zero rest.Middleware that validates the
// "Authorization: Bearer <token>" header, verifies the JWT, and injects the
// authenticated principal (and raw token) into the request context.
func JWTMiddleware(verifier AccessTokenVerifier) rest.Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				errors.WriteUnauthorized(w, "missing authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				errors.WriteUnauthorized(w, "invalid authorization format")
				return
			}

			tokenString := parts[1]

			claims, err := verifier.VerifyAccessToken(r.Context(), tokenString)
			if err != nil {
				errors.WriteUnauthorized(w, "invalid or expired token")
				return
			}

			p := principal.Principal{
				UserID:    claims.Subject.String(),
				Username:  claims.Username,
				Roles:     claims.Roles,
				SessionID: claims.SessionID.String(),
			}
			ctx := principal.WithPrincipal(r.Context(), p)
			ctx = principal.WithToken(ctx, tokenString)
			next(w, r.WithContext(ctx))
		}
	}
}
