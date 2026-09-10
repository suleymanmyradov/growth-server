package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeromicro/go-zero/rest"

	sharedmw "github.com/suleymanmyradov/growth-server/pkg/httpx/middleware"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/middleware"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
)

// newTestServer builds a serverless rest.Server with the generated routes and
// real auth middlewares, so route-level behavior can be asserted without
// binding a port or touching Postgres/RPC dependencies.
func newTestServer(t *testing.T) *rest.Serverless {
	t.Helper()

	server := rest.MustNewServer(rest.RestConf{Host: "127.0.0.1", Port: 0})
	svcCtx := &svc.ServiceContext{
		Auth: sharedmw.JWTMiddleware(sharedmw.JWTVerifierConfig{
			Secret:   strings.Repeat("t", 32),
			Issuer:   "growth-auth",
			Audience: "growth-api",
		}),
		AdminAuth: middleware.AdminAuth(),
	}
	RegisterHandlers(server, svcCtx)

	sl, err := rest.NewServerless(server)
	require.NoError(t, err)
	return sl
}

// The admin contract must not expose a public self-registration route: anyone
// could mint admin tokens. Regenerating from main.api must keep it removed.
func TestRoutes_NoPublicAdminRegistration(t *testing.T) {
	sl := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/register",
		strings.NewReader(`{"email":"attacker@example.com","password":"sup3rs3cret!","fullName":"Attacker","role":"admin"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	sl.Serve(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code, "public admin register must not exist, got body: %s", rec.Body.String())
}

// Login stays public (it is the only way in) but every other admin route must
// still require auth. Malformed bodies keep the requests in the handler layer
// (400) so no Postgres/RPC dependency is needed.
func TestRoutes_LoginPublicAndAdminRoutesProtected(t *testing.T) {
	sl := newTestServer(t)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/login", strings.NewReader(`{invalid`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	sl.Serve(loginRec, loginReq)
	assert.Equal(t, http.StatusBadRequest, loginRec.Code, "login route must exist (parse error, not 404)")

	articlesReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/articles", nil)
	articlesRec := httptest.NewRecorder()
	sl.Serve(articlesRec, articlesReq)
	assert.Equal(t, http.StatusUnauthorized, articlesRec.Code)
}
