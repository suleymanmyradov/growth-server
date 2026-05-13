// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package categories

import (
	"net/http"

	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/logic/categories"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ListCategoriesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ListCategoriesRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := categories.NewListCategoriesLogic(r.Context(), svcCtx)
		resp, err := l.ListCategories(&req)
		if err != nil {
			errors.HandleGrpcError(w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
