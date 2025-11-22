// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package habits

import (
	"net/http"

	"github.com/suleymanmyradov/growth-server/services/gateway/api/internal/logic/habits"
	"github.com/suleymanmyradov/growth-server/services/gateway/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ResetTodayHabitsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := habits.NewResetTodayHabitsLogic(r.Context(), svcCtx)
		resp, err := l.ResetTodayHabits()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
