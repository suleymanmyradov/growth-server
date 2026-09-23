// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package billing

import (
	"io"
	"net/http"

	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	"github.com/suleymanmyradov/growth-server/pkg/paddle"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/logic/billing"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func HandlePaddleWebhookHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Read raw body — signature verification happens in the client RPC
		// service, which owns the Paddle webhook secret. The gateway is just
		// a transport layer here (same pattern as the Stripe webhook).
		//
		// Limit the body to 1 MiB before reading it into memory. Paddle event
		// payloads are small JSON envelopes; anything larger is either a bug
		// or an abuse attempt and must be rejected before allocation.
		const maxPaddleBody = 1 << 20 // 1 MiB
		r.Body = http.MaxBytesReader(w, r.Body, maxPaddleBody)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			errors.HandleGrpcError(w, err)
			return
		}
		defer func() { _ = r.Body.Close() }()

		signature := r.Header.Get(paddle.SignatureHeader)
		if signature == "" {
			errors.WriteError(w, http.StatusBadRequest, "missing Paddle-Signature header")
			return
		}

		l := billing.NewHandlePaddleWebhookLogic(r.Context(), svcCtx)
		resp, err := l.HandlePaddleWebhook(body, signature)
		if err != nil {
			errors.HandleGrpcError(w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
