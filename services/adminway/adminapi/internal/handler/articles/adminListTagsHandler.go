// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package articles

import (
	"net/http"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/logic/articles"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func AdminListTagsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := articles.NewAdminListTagsLogic(r.Context(), svcCtx)
		resp, err := l.AdminListTags()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
