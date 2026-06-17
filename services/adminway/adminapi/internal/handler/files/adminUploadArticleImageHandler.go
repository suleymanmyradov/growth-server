package files

import (
	"io"
	"mime"
	"net/http"
	"path/filepath"

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

		resp, err := svcCtx.FileManagerRpc.UploadFile(r.Context(), &fileManagerClient.UploadFileRequest{
			Data:        data,
			Filename:    header.Filename,
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
