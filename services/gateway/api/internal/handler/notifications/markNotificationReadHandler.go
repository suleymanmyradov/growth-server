// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package notifications

import (
	"net/http"

	"github.com/suleymanmyradov/growth-server/services/gateway/api/internal/logic/notifications"
	"github.com/suleymanmyradov/growth-server/services/gateway/api/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func MarkNotificationReadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.NotificationRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := notifications.NewMarkNotificationReadLogic(r.Context(), svcCtx)
		resp, err := l.MarkNotificationRead(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
