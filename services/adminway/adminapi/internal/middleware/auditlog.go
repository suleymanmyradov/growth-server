// Audit logging for adminway: every request reaching the admin API is recorded
// in admin_audit_log — authenticated admin actions plus unauthenticated
// attempts (login, refresh, rejected tokens).
//
// Writes are buffered on a bounded channel and inserted by a single worker
// goroutine, so request latency never waits on Postgres and a DB outage only
// costs dropped audit rows (warned), never blocked requests.
package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/httpx/middleware"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/repository/db"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

const (
	auditQueueSize  = 512
	auditInsertTime = 3 * time.Second
	maxFieldLen     = 512
)

type auditEntry struct {
	db.InsertAdminAuditLogParams
}

// AuditLogger asynchronously persists request records.
type AuditLogger struct {
	queries  db.Querier
	verifier middleware.AccessTokenVerifier
	ch       chan auditEntry
	done     chan struct{}
	wg       sync.WaitGroup
}

// NewAuditLogger starts the drain worker. verifier is used ONLY to resolve
// which admin a Bearer token belongs to for the log row — authorization itself
// still happens in the route-level Auth/AdminAuth middlewares.
func NewAuditLogger(queries db.Querier, verifier middleware.AccessTokenVerifier) *AuditLogger {
	l := &AuditLogger{
		queries:  queries,
		verifier: verifier,
		ch:       make(chan auditEntry, auditQueueSize),
		done:     make(chan struct{}),
	}
	l.wg.Add(1)
	go l.worker()
	return l
}

// Middleware returns a rest.Middleware suitable for server.Use. Register it
// outermost-ish (before auth): it must see unauthenticated attempts too.
func (l *AuditLogger) Middleware() rest.Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				next(w, r)
				return
			}
			start := time.Now()
			sw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next(sw, r)

			var adminID uuid.NullUUID
			var adminName *string
			if tok := bearerToken(r); tok != "" {
				// The route middleware will reject a bad token regardless; here
				// we only resolve identity for the audit row. Failures are
				// expected (expired/forged tokens) — log the attempt anyway.
				if claims, err := l.verifier.VerifyAccessToken(r.Context(), tok); err == nil {
					adminID = uuid.NullUUID{UUID: claims.Subject, Valid: claims.Subject != uuid.Nil}
					if claims.Username != "" {
						name := truncate(claims.Username, 255)
						adminName = &name
					}
				}
			}

			ip := clientIP(r)
			ua := truncate(r.UserAgent(), maxFieldLen)
			l.enqueue(auditEntry{db.InsertAdminAuditLogParams{
				AdminID:    adminID,
				AdminName:  adminName,
				Method:     r.Method,
				Path:       truncate(r.URL.Path, maxFieldLen),
				StatusCode: int32(sw.status),
				Ip:         &ip,
				UserAgent:  &ua,
				LatencyMs:  int32(time.Since(start).Milliseconds()),
			}})
		}
	}
}

// Close drains the queue (best effort) and stops the worker.
func (l *AuditLogger) Close() {
	close(l.done)
	l.wg.Wait()
}

func (l *AuditLogger) enqueue(e auditEntry) {
	select {
	case l.ch <- e:
	default:
		logx.Errorf("audit log queue full — dropping audit entry for %s %s", e.Method, e.Path)
	}
}

func (l *AuditLogger) worker() {
	defer l.wg.Done()
	for {
		select {
		case e := <-l.ch:
			l.insert(e)
		case <-l.done:
			for {
				select {
				case e := <-l.ch:
					l.insert(e)
				default:
					return
				}
			}
		}
	}
}

func (l *AuditLogger) insert(e auditEntry) {
	ctx, cancel := context.WithTimeout(context.Background(), auditInsertTime)
	defer cancel()
	if err := l.queries.InsertAdminAuditLog(ctx, e.InsertAdminAuditLogParams); err != nil {
		logx.Errorf("audit log insert failed: %v", err)
	}
}

// statusRecorder captures the response status without breaking optional
// interfaces (http.Flusher) the wrapped writer may support.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusRecorder) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}
	return parts[1]
}

// clientIP prefers the Caddy-set X-Forwarded-For first hop; falls back to the
// direct peer address.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, ok := strings.Cut(xff, ","); ok {
			return truncate(strings.TrimSpace(first), 64)
		}
		return truncate(strings.TrimSpace(xff), 64)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return truncate(r.RemoteAddr, 64)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
