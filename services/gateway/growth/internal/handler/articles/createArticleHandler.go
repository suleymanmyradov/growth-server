package articles

import (
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/logic/articles"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func CreateArticleHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		var req types.CreateArticleRequest
		req.Title = r.FormValue("title")
		req.Content = r.FormValue("content")
		req.Summary = r.FormValue("summary")
		req.AuthorId = r.FormValue("authorId")
		req.CategoryId = r.FormValue("categoryId")
		req.Tags = r.Form["tags"]

		readTime, _ := strconv.Atoi(r.FormValue("readTime"))
		req.ReadTime = readTime

		var coverImageData []byte
		var coverImageFilename string
		var coverImageContentType string

		file, header, err := r.FormFile("coverImage")
		if err == nil {
			defer file.Close()
			coverImageData, _ = io.ReadAll(file)
			coverImageFilename = header.Filename
			coverImageContentType = header.Header.Get("Content-Type")
			if coverImageContentType == "" {
				ext := filepath.Ext(header.Filename)
				coverImageContentType = mime.TypeByExtension(ext)
				if coverImageContentType == "" {
					coverImageContentType = "application/octet-stream"
				}
			}
		}

		l := articles.NewCreateArticleLogic(r.Context(), svcCtx)
		resp, err := l.CreateArticle(&req, coverImageData, coverImageFilename, coverImageContentType)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
