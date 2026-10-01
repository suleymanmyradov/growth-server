package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/repository/db"
)

// fakeQuerier embeds the interface — only the method under test is overridden.
type fakeQuerier struct {
	db.Querier
	got chan db.InsertAdminAuditLogParams
	err error
}

func (f *fakeQuerier) InsertAdminAuditLog(_ context.Context, arg db.InsertAdminAuditLogParams) error {
	select {
	case f.got <- arg:
	default: // never block the drain worker in tests
	}
	return f.err
}

type stubVerifier struct {
	claims *jwt.TokenClaims
	err    error
}

func (s stubVerifier) VerifyAccessToken(context.Context, string) (*jwt.TokenClaims, error) {
	return s.claims, s.err
}

func serve(l *AuditLogger, method, path, authHeader, xff string, code int) {
	req := httptest.NewRequest(method, path, nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	rec := httptest.NewRecorder()
	l.Middleware()(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
	})(rec, req)
}

func TestAuditLogger_RecordsAuthenticatedRequest(t *testing.T) {
	q := &fakeQuerier{got: make(chan db.InsertAdminAuditLogParams, 1)}
	uid := uuid.New()
	v := stubVerifier{claims: &jwt.TokenClaims{Subject: uid, Username: "admin@x.io"}}
	l := NewAuditLogger(q, v)
	defer l.Close()

	serve(l, http.MethodDelete, "/api/v1/admin/articles/42", "Bearer tok", "203.0.113.9, 10.0.0.1", http.StatusNoContent)

	select {
	case row := <-q.got:
		assert.Equal(t, uid, row.AdminID.UUID)
		assert.True(t, row.AdminID.Valid)
		assert.Equal(t, "admin@x.io", *row.AdminName)
		assert.Equal(t, "DELETE", row.Method)
		assert.Equal(t, "/api/v1/admin/articles/42", row.Path)
		assert.Equal(t, int32(204), row.StatusCode)
		assert.Equal(t, "203.0.113.9", *row.Ip)
	case <-time.After(2 * time.Second):
		t.Fatal("audit row not inserted")
	}
}

func TestAuditLogger_LogsUnauthenticatedAttempt(t *testing.T) {
	q := &fakeQuerier{got: make(chan db.InsertAdminAuditLogParams, 1)}
	l := NewAuditLogger(q, stubVerifier{err: jwt.ErrInvalidToken})
	defer l.Close()

	serve(l, http.MethodPost, "/api/v1/admin/auth/login", "", "", http.StatusUnauthorized)

	select {
	case row := <-q.got:
		assert.False(t, row.AdminID.Valid)
		assert.Nil(t, row.AdminName)
		assert.Equal(t, int32(401), row.StatusCode)
		assert.Equal(t, "POST", row.Method)
	case <-time.After(2 * time.Second):
		t.Fatal("audit row not inserted")
	}
}

func TestAuditLogger_DropsWhenQueueFull(t *testing.T) {
	// Blocking fake: worker never drains.
	q := &fakeQuerier{got: make(chan db.InsertAdminAuditLogParams)}
	l := NewAuditLogger(q, stubVerifier{err: jwt.ErrInvalidToken})
	defer l.Close()

	done := make(chan struct{})
	go func() {
		for i := 0; i < auditQueueSize+50; i++ {
			serve(l, http.MethodGet, "/api/v1/admin/x", "", "", 200)
		}
		close(done)
	}()
	select {
	case <-done: // must not block even with a saturated queue
	case <-time.After(5 * time.Second):
		t.Fatal("requests blocked on full audit queue")
	}
}
