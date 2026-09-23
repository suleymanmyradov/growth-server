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
		//
		// Limit the body to 1 MiB before reading it into memory. Stripe event
		// payloads are small JSON envelopes; anything larger is either a bug or
		// an abuse attempt and must be rejected before allocation. The go-zero
		// MaxBytes middleware only checks Content-Length, so chunked requests
		// would bypass it without this cap (same as the RevenueCat webhook).
		const maxStripeBody = 1 << 20 // 1 MiB
		r.Body = http.MaxBytesReader(w, r.Body, maxStripeBody)
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
