package files

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/suleymanmyradov/growth-server/pkg/imageproc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/fileManagerClient"
	"github.com/zeromicro/go-zero/rest/httpx"
)

type UploadImageResponse struct {
	Url string `json:"url"`
	Key string `json:"key"`
}

func AdminUploadArticleImageHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		// Resize, convert to JPEG, and enforce size limits.
		data, contentType, err := imageproc.ProcessArticleCover(file)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		// Force .jpg extension so the stored key matches the processed format.
		filename := header.Filename
		if ext := filepath.Ext(filename); ext != "" {
			filename = strings.TrimSuffix(filename, ext) + ".jpg"
		} else {
			filename = filename + ".jpg"
		}

		resp, err := svcCtx.FileManagerRpc.UploadFile(r.Context(), &fileManagerClient.UploadFileRequest{
			Data:        data,
			Filename:    filename,
			ContentType: contentType,
			Folder:      "articles",
		})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		httpx.OkJsonCtx(r.Context(), w, UploadImageResponse{
			Url: resp.Url,
			Key: resp.Key,
		})
	}
}
