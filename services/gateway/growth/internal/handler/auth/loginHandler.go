// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package auth

import (
	"net/http"

	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/logic/auth"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func LoginHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LoginRequest
		if err := httpx.Parse(r, &req); err != nil {
			errors.WriteParseError(w, err)
			return
		}

		l := auth.NewLoginLogic(r.Context(), svcCtx)
		resp, err := l.Login(&req)
		if err != nil {
			// Wrong credentials return Unauthenticated with a user-facing message
			// ("invalid email or password"). HandleGrpcError would sanitize that to
			// the generic "authentication required", which is confusing on a login
			// form — write the original message directly instead.
			if st, ok := status.FromError(err); ok && st.Code() == codes.Unauthenticated {
				errors.WriteError(w, http.StatusUnauthorized, st.Message())
				return
			}
			errors.HandleGrpcError(w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
