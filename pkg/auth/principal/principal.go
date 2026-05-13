package principal

import "context"

type ctxKey struct{}
type tokenKey struct{}

// Principal represents the authenticated user context with identity and role information.
type Principal struct {
	UserID    string
	Username  string
	Roles     []string
	SessionID string
}

// WithPrincipal stores the Principal in the context for downstream use.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// PrincipalFrom retrieves the Principal from the context.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}

// WithToken stores a JWT token in the context for downstream use.
func WithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenKey{}, token)
}

// TokenFrom retrieves the JWT token from the context.
func TokenFrom(ctx context.Context) (string, bool) {
	t, ok := ctx.Value(tokenKey{}).(string)
	return t, ok
}
