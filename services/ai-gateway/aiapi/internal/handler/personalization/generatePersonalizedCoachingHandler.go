// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package personalization

import (
	"net/http"

	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"

	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/logic/personalization"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GeneratePersonalizedCoachingHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GeneratePersonalizedCoachingRequest
		if err := httpx.Parse(r, &req); err != nil {
			errors.WriteParseError(w, err)
			return
		}

		l := personalization.NewGeneratePersonalizedCoachingLogic(r.Context(), svcCtx)
		resp, err := l.GeneratePersonalizedCoaching(&req)
		if err != nil {
			errors.HandleGrpcError(w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
