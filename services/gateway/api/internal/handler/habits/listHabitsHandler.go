// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package habits

import (
	"net/http"

	"github.com/suleymanmyradov/growth-server/services/gateway/api/internal/logic/habits"
	"github.com/suleymanmyradov/growth-server/services/gateway/api/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ListHabitsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.PageRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := habits.NewListHabitsLogic(r.Context(), svcCtx)
		resp, err := l.ListHabits(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
