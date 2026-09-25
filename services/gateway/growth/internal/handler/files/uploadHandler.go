package files

import (
	"io"
	"mime"
	"net/http"
	"path/filepath"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/fileManagerClient"
	"github.com/zeromicro/go-zero/rest/httpx"
)

type UploadResponse struct {
	Url string `json:"url"`
	Key string `json:"key"`
}

func UploadFileHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Limit upload size to 10 MB
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			httpx.Error(w, err)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			httpx.Error(w, err)
			return
		}
		defer func() { _ = file.Close() }()

		folder := r.FormValue("folder")
		if folder == "" {
			folder = "uploads"
		}

		data, err := io.ReadAll(file)
		if err != nil {
			httpx.Error(w, err)
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

		// The route is behind ctx.Auth, so the principal is always present;
		// passing its ID records the caller as the object's owner in the
		// filemanager registry.
		var userID string
		if p, ok := principal.PrincipalFrom(r.Context()); ok {
			userID = p.UserID
		}

		resp, err := svcCtx.FileManagerRpc.UploadFile(r.Context(), &fileManagerClient.UploadFileRequest{
			Data:        data,
			Filename:    header.Filename,
			ContentType: contentType,
			Folder:      folder,
			UserId:      userID,
		})
		if err != nil {
			httpx.Error(w, err)
			return
		}

		httpx.OkJsonCtx(r.Context(), w, UploadResponse{
			Url: resp.Url,
			Key: resp.Key,
		})
	}
}
