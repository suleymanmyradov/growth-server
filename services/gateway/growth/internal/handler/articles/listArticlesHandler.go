// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package articles

import (
	"context"
	"net/http"
	"strings"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/logic/articles"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ListArticlesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ListArticlesRequest
		if err := httpx.Parse(r, &req); err != nil {
			errors.WriteParseError(w, err)
			return
		}

		ctx := optionalAuth(r, svcCtx)

		l := articles.NewListArticlesLogic(ctx, svcCtx)
		resp, err := l.ListArticles(&req)
		if err != nil {
			errors.HandleGrpcError(w, err)
		} else {
			httpx.OkJsonCtx(ctx, w, resp)
		}
	}
}

func optionalAuth(r *http.Request, svcCtx *svc.ServiceContext) context.Context {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return r.Context()
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return r.Context()
	}

	if svcCtx.TokenMaker == nil {
		return r.Context()
	}

	claims, err := svcCtx.TokenMaker.VerifyAccessToken(r.Context(), parts[1])
	if err != nil {
		return r.Context()
	}

	p := principal.Principal{
		UserID:    claims.Subject.String(),
		Username:  claims.Username,
		Roles:     claims.Roles,
		SessionID: claims.SessionID.String(),
	}
	ctx := principal.WithPrincipal(r.Context(), p)
	ctx = principal.WithToken(ctx, parts[1])
	return ctx
}
