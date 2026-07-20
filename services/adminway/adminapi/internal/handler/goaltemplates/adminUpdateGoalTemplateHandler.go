// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package goaltemplates

import (
	"net/http"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/logic/goaltemplates"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func AdminUpdateGoalTemplateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UpdateGoalTemplateRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := goaltemplates.NewAdminUpdateGoalTemplateLogic(r.Context(), svcCtx)
		resp, err := l.AdminUpdateGoalTemplate(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
