// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package profile

import (
	"net/http"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/logic/profile"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ExportDataHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := profile.NewExportDataLogic(r.Context(), svcCtx)
		resp, err := l.ExportData()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
