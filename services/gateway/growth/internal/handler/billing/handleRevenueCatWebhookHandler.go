// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package billing

import (
	"io"
	"net/http"

	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/logic/billing"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func HandleRevenueCatWebhookHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Read raw body — signature verification happens in the client RPC
		// service, which owns the RevenueCat webhook secret. The gateway is
		// just a transport layer here (same pattern as the Stripe webhook).
		//
		// Limit the body to 1 MiB before reading it into memory. RevenueCat
		// webhook payloads are small JSON envelopes; anything larger is either
		// a bug or an abuse attempt and must be rejected before allocation.
		const maxRevenueCatBody = 1 << 20 // 1 MiB
		r.Body = http.MaxBytesReader(w, r.Body, maxRevenueCatBody)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			errors.HandleGrpcError(w, err)
			return
		}
		defer r.Body.Close()

		authorization := r.Header.Get("Authorization")
		if authorization == "" {
			errors.WriteError(w, http.StatusUnauthorized, "missing Authorization header")
			return
		}

		l := billing.NewHandleRevenueCatWebhookLogic(r.Context(), svcCtx)
		resp, err := l.HandleRevenueCatWebhook(body, authorization)
		if err != nil {
			errors.HandleGrpcError(w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
