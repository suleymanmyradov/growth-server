package personalization

import (
	"net/http"

	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/logic/personalization"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// GenerateOnboardingHabitsHandler is a server-owned endpoint that generates 3
// daily habit suggestions from structured onboarding data. The client never
// supplies a prompt or tools — only the fields in GenerateOnboardingHabitsRequest.
// The ai-coach backend builds the prompt, runs the safety classifier, and
// returns validated structured JSON.
func GenerateOnboardingHabitsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GenerateOnboardingHabitsRequest
		if err := httpx.Parse(r, &req); err != nil {
			errors.WriteParseError(w, err)
			return
		}

		l := personalization.NewGenerateOnboardingHabitsLogic(r.Context(), svcCtx)
		resp, err := l.GenerateOnboardingHabits(&req)
		if err != nil {
			errors.HandleGrpcError(w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
