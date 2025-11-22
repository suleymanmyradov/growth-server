package middleware

import (
	"context"
	"net/http"
	"strings"

	authpb "github.com/suleymanmyradov/growth-server/services/gateway/services/auth/rpc/auth"
	authclient "github.com/suleymanmyradov/growth-server/services/gateway/services/auth/rpc/authClient"
)

const (
	authorizationHeaderKey = "Authorization"
	bearerPrefix           = "Bearer "
)

type RequiredAuthMiddleware struct {
	auth authclient.Auth
}

type OptionalAuthMiddleware struct {
	auth authclient.Auth
}

func NewRequiredAuthMiddleware(authClient authclient.Auth) *RequiredAuthMiddleware {
	return &RequiredAuthMiddleware{auth: authClient}
}

func NewOptionalAuthMiddleware(authClient authclient.Auth) *OptionalAuthMiddleware {
	return &OptionalAuthMiddleware{auth: authClient}
}

func (m *RequiredAuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get(authorizationHeaderKey)
		if authHeader == "" || !strings.HasPrefix(authHeader, bearerPrefix) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("unauthorized"))
			return
		}

		token := strings.TrimPrefix(authHeader, bearerPrefix)

		resp, err := m.auth.ValidateToken(r.Context(), &authpb.ValidateTokenRequest{AccessToken: token})
		if err != nil || resp == nil || !resp.Valid {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("unauthorized"))
			return
		}

		ctx := context.WithValue(r.Context(), "userId", resp.UserId)
		next(w, r.WithContext(ctx))
	}
}

func (m *OptionalAuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get(authorizationHeaderKey)
		if authHeader == "" || !strings.HasPrefix(authHeader, bearerPrefix) {
			next(w, r)
			return
		}

		token := strings.TrimPrefix(authHeader, bearerPrefix)

		resp, err := m.auth.ValidateToken(r.Context(), &authpb.ValidateTokenRequest{AccessToken: token})
		if err != nil || resp == nil || !resp.Valid {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("unauthorized"))
			return
		}

		ctx := context.WithValue(r.Context(), "userId", resp.UserId)
		next(w, r.WithContext(ctx))
	}
}
