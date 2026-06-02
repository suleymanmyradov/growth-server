// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package billing

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/logic/billing"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func HandleStripeWebhookHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Read raw body for signature verification
		body, err := io.ReadAll(r.Body)
		if err != nil {
			errors.HandleGrpcError(w, err)
			return
		}
		defer r.Body.Close()

		signature := r.Header.Get("Stripe-Signature")
		if signature == "" {
			errors.WriteError(w, http.StatusBadRequest, "missing Stripe-Signature header")
			return
		}

		// Verify webhook signature at the gateway edge
		if svcCtx.StripeClient != nil && svcCtx.Config.Billing.StripeWebhookSecret != "" {
			eventType, verifyErr := svcCtx.StripeClient.VerifyWebhookSignature(body, signature, svcCtx.Config.Billing.StripeWebhookSecret)
			if verifyErr != nil {
				logx.WithContext(r.Context()).Errorf("Stripe webhook verification failed: %v", verifyErr)
				errors.WriteError(w, http.StatusBadRequest, "invalid signature")
				return
			}
			// Parse the verified payload to extract event data
			var payload map[string]interface{}
			if jsonErr := json.Unmarshal(body, &payload); jsonErr != nil {
				errors.WriteParseError(w, jsonErr)
				return
			}
			dataObj, ok := payload["data"]
			if !ok {
				errors.WriteError(w, http.StatusBadRequest, "missing data field")
				return
			}
			dataJSON, marshalErr := json.Marshal(dataObj)
			if marshalErr != nil {
				errors.WriteParseError(w, marshalErr)
				return
			}

			req := types.StripeWebhookRequest{
				EventType:   eventType,
				PayloadJson: string(dataJSON),
			}

			l := billing.NewHandleStripeWebhookLogic(r.Context(), svcCtx)
			resp, err := l.HandleStripeWebhook(&req)
			if err != nil {
				errors.HandleGrpcError(w, err)
			} else {
				httpx.OkJsonCtx(r.Context(), w, resp)
			}
			return
		}

		// Fallback: if no stripe client configured, still parse and forward
		var req types.StripeWebhookRequest
		if err := json.Unmarshal(body, &req); err != nil {
			errors.HandleGrpcError(w, err)
			return
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
