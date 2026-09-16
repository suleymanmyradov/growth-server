// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package articles

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/fileManagerClient"

	"github.com/zeromicro/go-zero/core/logx"
)

// UploadCtxKey is a context key used to pass parsed file data from the
// handler to the logic layer without modifying the generated types.
type UploadCtxKey struct{}

// UploadFileData holds the parsed multipart form fields passed from the handler.
type UploadFileData struct {
	Data     []byte
	Filename string
}

// allowedImageTypes is the content-type allowlist for article images, matched
// against the type detected from the actual bytes — never the client-declared
// header. The filemanager RPC enforces the same policy at storage time.
var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

// detectImageType sniffs the payload's magic bytes and returns the detected
// content type, or an error if it is not an allowed image type.
func detectImageType(data []byte) (string, error) {
	if len(data) == 0 {
		return "", status.Error(codes.InvalidArgument, "empty file")
	}
	sniff := data
	if len(sniff) > 512 {
		sniff = sniff[:512]
	}
	detected := http.DetectContentType(sniff)
	if i := strings.Index(detected, ";"); i != -1 {
		detected = strings.TrimSpace(detected[:i])
	}
	if !allowedImageTypes[detected] {
		return "", status.Error(codes.InvalidArgument, fmt.Sprintf("file type %q is not allowed", detected))
	}
	return detected, nil
}

type AdminUploadArticleImageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminUploadArticleImageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUploadArticleImageLogic {
	return &AdminUploadArticleImageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminUploadArticleImageLogic) AdminUploadArticleImage(req *types.UploadImageRequest) (resp *types.UploadImageResponse, err error) {
	data, ok := l.ctx.Value(UploadCtxKey{}).(*UploadFileData)
	if !ok || data == nil {
		return nil, fmt.Errorf("no file data in context")
	}

	contentType, err := detectImageType(data.Data)
	if err != nil {
		return nil, err
	}

	rpcResp, err := l.svcCtx.FileManagerRpc.UploadFile(l.ctx, &fileManagerClient.UploadFileRequest{
		Data:        data.Data,
		Filename:    data.Filename,
		ContentType: contentType,
		Folder:      "articles",
	})
	if err != nil {
		l.Errorf("Failed to upload article image: %v", err)
		return nil, err
	}

	return &types.UploadImageResponse{
		Url: rpcResp.Url,
		Key: rpcResp.Key,
	}, nil
}
