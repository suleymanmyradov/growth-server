package personalization

import (
	"net/http"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/logic/personalization"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/sse"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StreamPersonalizedCoachingHandler is an SSE endpoint that streams the
// AI-generated personalized coaching response as it is produced.
//
// The agentic path (StreamAgent with on-demand tool calls) is the only path.
// The model fetches user data (goals, habits, check-ins, etc.) only when the
// user's message makes it relevant, instead of eagerly stuffing everything
// into the prompt.
//
// SSE event types:
//   - reasoning: {"text": "..."} — model's live reasoning/thinking deltas (reasoning models only)
//   - thinking:  {"message": "..."} — status updates while tools execute / model processes
//   - delta:     {"text": "..."} — incremental coaching text
//   - proposal:  {id, action, payload} — a confirm/cancel card for a proposed CRUD action
//   - complete:  {"fullResponse": "..."} — final full response
//   - error:     {"message": "..."} — error before stream end
//
// The handler is a thin wrapper: request parsing, auth, and dependency
// wiring. All orchestration lives in personalization.StreamCoaching.
func StreamPersonalizedCoachingHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GeneratePersonalizedCoachingRequest
		if err := httpx.Parse(r, &req); err != nil {
			errors.WriteParseError(w, err)
			return
		}

		p, ok := principal.PrincipalFrom(r.Context())
		if !ok {
			errors.HandleGrpcError(w, status.Error(codes.Unauthenticated, "not authenticated"))
			return
		}

		personalization.StreamCoaching(r.Context(), sse.NewWriter(w), &req, p, personalization.StreamCoachingDeps{
			AIClient:       svcCtx.AIClient,
			Classifier:     svcCtx.Classifier,
			Conversations:  svcCtx.AICoachRpc.ConversationService,
			ProfileFetcher: svcCtx.AuthRpc,
			ToolDeps: personalization.CoachingToolDeps{
				Goals:           svcCtx.ClientRpc.Goals,
				Habits:          svcCtx.ClientRpc.Habits,
				CheckIns:        svcCtx.ClientRpc.CheckInService,
				WeeklyReviews:   svcCtx.ClientRpc.WeeklyReviewService,
				Personalization: svcCtx.ClientRpc.PersonalizationService,
				Search:          svcCtx.SearchRpc,
				Articles:        svcCtx.ClientRpc.Articles,
				Memory:          svcCtx.AICoachRpc.AICoachService,
			},
			Config: svcCtx.Config.Coaching,
		})
	}
}
