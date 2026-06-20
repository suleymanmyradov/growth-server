// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package articles

import (
	"context"
	"io"
	"mime"
	"net/http"
	"path/filepath"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/logic/articles"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func AdminUploadArticleImageHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Limit upload size to 10 MB
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		file, header, err := r.FormFile("image")
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		contentType := header.Header.Get("Content-Type")
		if contentType == "" {
			ext := filepath.Ext(header.Filename)
			contentType = mime.TypeByExtension(ext)
			if contentType == "" {
				contentType = "application/octet-stream"
			}
		}

		// Pass file data to the logic layer via context.
		ctx := context.WithValue(r.Context(), articles.UploadCtxKey{}, &articles.UploadFileData{
			Data:        data,
			Filename:    header.Filename,
			ContentType: contentType,
		})

		l := articles.NewAdminUploadArticleImageLogic(ctx, svcCtx)
		resp, err := l.AdminUploadArticleImage(&types.UploadImageRequest{})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
