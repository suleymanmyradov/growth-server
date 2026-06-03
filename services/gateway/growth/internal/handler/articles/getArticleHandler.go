// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package articles

import (
	"net/http"

	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/logic/articles"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetArticleHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ArticleRequest
		if err := httpx.Parse(r, &req); err != nil {
			errors.WriteParseError(w, err)
			return
		}

		ctx := optionalAuth(r, svcCtx)

		l := articles.NewGetArticleLogic(ctx, svcCtx)
		resp, err := l.GetArticle(&req)
		if err != nil {
			errors.HandleGrpcError(w, err)
		} else {
			httpx.OkJsonCtx(ctx, w, resp)
		}
	}
}
