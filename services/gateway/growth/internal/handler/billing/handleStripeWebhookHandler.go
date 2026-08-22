// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package billing

import (
	"io"
	"net/http"

	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/logic/billing"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func HandleStripeWebhookHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Read raw body — signature verification happens in the client RPC service,
		// which owns all Stripe secrets. The gateway is just a transport layer here.
		body, err := io.ReadAll(r.Body)
		if err != nil {
			errors.HandleGrpcError(w, err)
			return
		}
		defer func() { _ = r.Body.Close() }()

		signature := r.Header.Get("Stripe-Signature")
		if signature == "" {
			errors.WriteError(w, http.StatusBadRequest, "missing Stripe-Signature header")
			return
		}

		req := types.StripeWebhookRequest{
			RawBody:   string(body),
			Signature: signature,
		}

		l := billing.NewHandleStripeWebhookLogic(r.Context(), svcCtx)
		resp, err := l.HandleStripeWebhook(&req)
		if err != nil {
			errors.HandleGrpcError(w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
