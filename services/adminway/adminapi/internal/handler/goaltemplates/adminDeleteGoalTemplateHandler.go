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

func AdminDeleteGoalTemplateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DeleteGoalTemplateRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := goaltemplates.NewAdminDeleteGoalTemplateLogic(r.Context(), svcCtx)
		resp, err := l.AdminDeleteGoalTemplate(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
