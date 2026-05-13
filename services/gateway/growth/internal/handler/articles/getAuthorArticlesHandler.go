// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package articles

import (
	"net/http"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/logic/articles"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetAuthorArticlesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GetAuthorArticlesRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := articles.NewGetAuthorArticlesLogic(r.Context(), svcCtx)
		resp, err := l.GetAuthorArticles(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
