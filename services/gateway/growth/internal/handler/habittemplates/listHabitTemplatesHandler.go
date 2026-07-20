// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package habittemplates

import (
	"net/http"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/logic/habittemplates"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ListHabitTemplatesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := habittemplates.NewListHabitTemplatesLogic(r.Context(), svcCtx)
		resp, err := l.ListHabitTemplates()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
