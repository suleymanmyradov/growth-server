package logic

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/pb/filemanager"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

// uploadPolicy allowlists what each upload folder may contain, keyed by the
// content type detected from the actual bytes (http.DetectContentType), never
// the client-declared header. Uploaded objects are served from the bucket, so
// scriptable types (HTML, SVG) must never be accepted. Folders not listed are
// rejected outright — new upload surfaces must be added here explicitly.
var uploadPolicy = map[string]map[string]bool{
	"avatars":  {"image/jpeg": true, "image/png": true, "image/webp": true, "image/gif": true},
	"articles": {"image/jpeg": true, "image/png": true, "image/webp": true, "image/gif": true},
	"exports":  {"text/plain": true}, // JSON export payloads sniff as text/plain
}

// validateUpload enforces the per-folder content-type allowlist. It sniffs the
// payload so a mislabeled HTML/SVG file cannot pass as an image.
func validateUpload(folder string, data []byte) error {
	allowed, ok := uploadPolicy[folder]
	if !ok {
		return fmt.Errorf("folder %q is not permitted for uploads", folder)
	}

	sniff := data
	if len(sniff) > 512 {
		sniff = sniff[:512]
	}
	detected := http.DetectContentType(sniff)
	if i := strings.Index(detected, ";"); i != -1 {
		detected = strings.TrimSpace(detected[:i])
	}
	if !allowed[detected] {
		return fmt.Errorf("content type %q is not allowed in folder %q", detected, folder)
	}
	return nil
}

type UploadFileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUploadFileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UploadFileLogic {
	return &UploadFileLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UploadFileLogic) UploadFile(in *filemanager.UploadFileRequest) (*filemanager.UploadFileResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "UploadFileLogic.UploadFile")
	defer span.End()

	bucket := l.svcCtx.Config.MinIO.DefaultBucket
	if bucket == "" {
		return nil, fmt.Errorf("minio default bucket not configured")
	}

	if err := validateUpload(in.Folder, in.Data); err != nil {
		logx.WithContext(ctx).Errorf("upload rejected: %v", err)
		return nil, fmt.Errorf("upload rejected: %w", err)
	}

	ext := filepath.Ext(in.Filename)
	key := fmt.Sprintf("%s/%s%s", in.Folder, uuid.New().String(), ext)

	_, err := l.svcCtx.Minio.PutObject(ctx, bucket, key, bytes.NewReader(in.Data), int64(len(in.Data)), minio.PutObjectOptions{
		ContentType: in.ContentType,
	})
	if err != nil {
		logx.WithContext(ctx).Errorf("minio put object failed: %v", err)
		return nil, fmt.Errorf("upload failed: %w", err)
	}

	var url string
	if publicBase := l.svcCtx.Config.MinIO.PublicBaseUrl; publicBase != "" {
		url = fmt.Sprintf("%s/%s/%s", publicBase, bucket, key)
	} else if l.svcCtx.Config.MinIO.UseSSL {
		url = fmt.Sprintf("https://%s/%s/%s", l.svcCtx.Config.MinIO.Endpoint, bucket, key)
	} else {
		url = fmt.Sprintf("http://%s/%s/%s", l.svcCtx.Config.MinIO.Endpoint, bucket, key)
	}

	logx.WithContext(ctx).Infof("uploaded file to %s", url)
	return &filemanager.UploadFileResponse{
		Url: url,
		Key: key,
	}, nil
}
