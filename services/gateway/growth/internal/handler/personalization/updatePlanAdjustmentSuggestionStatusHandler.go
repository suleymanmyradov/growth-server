// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package personalization

import (
	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	"net/http"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/logic/personalization"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func UpdatePlanAdjustmentSuggestionStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UpdatePlanAdjustmentSuggestionStatusRequest
		if err := httpx.Parse(r, &req); err != nil {
			errors.WriteParseError(w, err)
			return
		}

		l := personalization.NewUpdatePlanAdjustmentSuggestionStatusLogic(r.Context(), svcCtx)
		resp, err := l.UpdatePlanAdjustmentSuggestionStatus(&req)
		if err != nil {
			errors.HandleGrpcError(w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
