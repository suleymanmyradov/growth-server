// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package goals

import (
	"net/http"

	"gateway/internal/logic/goals"
	"gateway/internal/svc"
	"gateway/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func UpdateGoalProgressHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UpdateProgressRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := goals.NewUpdateGoalProgressLogic(r.Context(), svcCtx)
		resp, err := l.UpdateGoalProgress(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
