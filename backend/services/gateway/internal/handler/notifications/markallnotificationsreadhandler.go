// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package notifications

import (
	"net/http"

	"gateway/internal/logic/notifications"
	"gateway/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func MarkAllNotificationsReadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := notifications.NewMarkAllNotificationsReadLogic(r.Context(), svcCtx)
		resp, err := l.MarkAllNotificationsRead()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
