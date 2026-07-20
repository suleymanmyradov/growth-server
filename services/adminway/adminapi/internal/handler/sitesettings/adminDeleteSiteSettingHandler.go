// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package sitesettings

import (
	"net/http"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/logic/sitesettings"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func AdminDeleteSiteSettingHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DeleteSiteSettingRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := sitesettings.NewAdminDeleteSiteSettingLogic(r.Context(), svcCtx)
		resp, err := l.AdminDeleteSiteSetting(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
