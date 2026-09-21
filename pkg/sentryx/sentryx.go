// Package sentryx wires Sentry error tracking into the services.
//
// Sentry is enabled when the SENTRY_DSN env var is set (all prod containers
// get it via env_file .env.prod); every helper is a safe no-op when it is
// empty, so local dev and tests are unaffected.
//
// Env vars:
//
//	SENTRY_DSN         — project DSN from sentry.io (required to enable)
//	SENTRY_ENVIRONMENT — issue environment label (default "production")
//	SENTRY_RELEASE     — optional release/deploy identifier
package sentryx

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/zeromicro/go-zero/core/logx"
)

const flushTimeout = 2 * time.Second

// Init initializes Sentry for this process. service is a short identifier
// ("gateway", "auth", ...) sent as the server name on every event.
// Returns whether Sentry was enabled.
func Init(service string) bool {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		return false
	}
	env := os.Getenv("SENTRY_ENVIRONMENT")
	if env == "" {
		env = "production"
	}
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      env,
		Release:          os.Getenv("SENTRY_RELEASE"),
		ServerName:       service,
		AttachStacktrace: true,
		// Distributed tracing already goes to Tempo; keep a small slice here
		// purely for performance context around errors.
		TracesSampleRate: 0.05,
		// Never send request bodies — they can carry credentials/PII.
		SendDefaultPII: false,
	}); err != nil {
		logx.Errorf("sentryx: init failed: %v", err)
		return false
	}
	logx.Infof("sentryx: enabled (env=%s)", env)
	return true
}

// Flush drains buffered events. Defer in main() after Init.
func Flush() {
	sentry.Flush(flushTimeout)
}

// Capture reports a recovered panic value (from a recover() call site) with
// the request's context attached. No-op when Sentry is disabled.
func Capture(ctx context.Context, recovered any) {
	hub := sentry.CurrentHub().Clone()
	hub.RecoverWithContext(ctx, recovered)
}

// Middleware returns an HTTP middleware that captures handler panics to
// Sentry, then re-panics so the outer server still produces a 500.
// Register with server.Use(...) on the REST gateways.
func Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					hub := sentry.CurrentHub().Clone()
					hub.RecoverWithContext(r.Context(), rec)
					hub.Flush(flushTimeout)
					panic(rec)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
