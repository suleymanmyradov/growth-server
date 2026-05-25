package weeklyreview

import (
	"net/http"

	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/logic/weeklyreview"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetCurrentWeeklyReviewHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := weeklyreview.NewGetCurrentWeeklyReviewLogic(r.Context(), svcCtx)
		resp, err := l.GetCurrentWeeklyReview()
		if err != nil {
			errors.HandleGrpcError(w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
