package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	"github.com/zeromicro/go-zero/rest"
)

type JWTVerifierConfig struct {
	Secret   string
	Issuer   string
	Audience string
}

func JWTMiddleware(cfg JWTVerifierConfig) rest.Middleware {
	maker, err := jwt.NewTokenMaker(jwt.Config{
		Secret:   cfg.Secret,
		Issuer:   cfg.Issuer,
		Audience: cfg.Audience,
	}, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create token verifier: %v", err))
	}

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

			claims, err := maker.VerifyAccessToken(r.Context(), tokenString)
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
