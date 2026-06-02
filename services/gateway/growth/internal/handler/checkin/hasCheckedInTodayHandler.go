package checkin

import (
	"net/http"

	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/logic/checkin"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func HasCheckedInTodayHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.HasCheckedInTodayRequest
		if err := httpx.Parse(r, &req); err != nil {
			errors.WriteParseError(w, err)
			return
		}

		l := checkin.NewHasCheckedInTodayLogic(r.Context(), svcCtx)
		resp, err := l.HasCheckedInToday(&req)
		if err != nil {
			errors.HandleGrpcError(w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
