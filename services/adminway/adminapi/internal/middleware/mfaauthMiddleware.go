// MfaAuth guards the MFA setup/confirm endpoints, which are reachable in two
// states: an admin with a full session enrolling voluntarily (Bearer JWT), or
// an admin mid-login forced to enroll (enroll-purpose ticket in the
// X-Mfa-Ticket header, minted by /auth/login when Mfa.Required is on).
// Both resolve to a principal so the logic layer reads identity the same way.
// The validated ticket row is also put into context so confirm can consume it.
package middleware

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	sharedmw "github.com/suleymanmyradov/growth-server/pkg/httpx/middleware"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/mfa"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/repository/db"
	"github.com/zeromicro/go-zero/rest"
)

// MfaTicketHeader carries the enroll-purpose ticket on setup/confirm calls.
const MfaTicketHeader = "X-Mfa-Ticket"

type mfaTicketCtxKey struct{}

// mfaTicketStore is the narrow read surface the middleware needs — *db.Queries
// satisfies it.
type mfaTicketStore interface {
	GetAdminMfaTicketByHash(ctx context.Context, tokenHash string) (db.AdminMfaTicket, error)
}

// WithMfaTicket stores the validated ticket row for the logic layer.
func WithMfaTicket(ctx context.Context, t db.AdminMfaTicket) context.Context {
	return context.WithValue(ctx, mfaTicketCtxKey{}, t)
}

// MfaTicketFrom retrieves the enroll ticket the request was authorized with
// (nil when the caller used a normal admin JWT).
func MfaTicketFrom(ctx context.Context) (db.AdminMfaTicket, bool) {
	t, ok := ctx.Value(mfaTicketCtxKey{}).(db.AdminMfaTicket)
	return t, ok
}

// MfaAuth accepts an admin Bearer token or a valid enroll-purpose MFA ticket.
// A verify-purpose ticket is never accepted here — it has not completed the
// second factor and must not reach endpoints that can change it.
func MfaAuth(verifier sharedmw.AccessTokenVerifier, tickets mfaTicketStore) rest.Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
				bearer := strings.TrimPrefix(authHeader, "Bearer ")
				claims, err := verifier.VerifyAccessToken(r.Context(), bearer)
				if err != nil {
					errors.WriteUnauthorized(w, "invalid or expired token")
					return
				}
				if !slices.Contains(claims.Roles, "admin") {
					errors.WriteUnauthorized(w, "admin role required")
					return
				}
				ctx := principal.WithPrincipal(r.Context(), principal.Principal{
					UserID:    claims.Subject.String(),
					Username:  claims.Username,
					Roles:     claims.Roles,
					SessionID: claims.SessionID.String(),
				})
				ctx = principal.WithToken(ctx, bearer)
				next(w, r.WithContext(ctx))
				return
			}

			if ticket := r.Header.Get(MfaTicketHeader); ticket != "" {
				row, err := tickets.GetAdminMfaTicketByHash(r.Context(), mfa.HashTicket(ticket))
				if err != nil || row.Purpose != mfa.TicketPurposeEnroll || time.Now().After(row.ExpiresAt.Time) {
					errors.WriteUnauthorized(w, "invalid or expired mfa ticket")
					return
				}
				ctx := principal.WithPrincipal(r.Context(), principal.Principal{
					UserID: row.UserID.String(),
				})
				ctx = WithMfaTicket(ctx, row)
				next(w, r.WithContext(ctx))
				return
			}

			errors.WriteUnauthorized(w, "authentication required")
		}
	}
}
