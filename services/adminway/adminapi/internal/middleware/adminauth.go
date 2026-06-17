package middleware

import (
	"net/http"
	"slices"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	"github.com/zeromicro/go-zero/rest"
)

func AdminAuth() rest.Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			p, ok := principal.PrincipalFrom(r.Context())
			if !ok {
				errors.WriteUnauthorized(w, "authentication required")
				return
			}

			if !slices.Contains(p.Roles, "admin") {
				errors.WriteUnauthorized(w, "admin role required")
				return
			}

			next(w, r)
		}
	}
}
