// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package conversations

import (
	"net/http"

	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/logic/conversations"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ListMemoryFactsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ListMemoryFactsRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := conversations.NewListMemoryFactsLogic(r.Context(), svcCtx)
		resp, err := l.ListMemoryFacts(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
