// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package articles

import (
	"context"
	"fmt"

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
	Data        []byte
	Filename    string
	ContentType string
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

	rpcResp, err := l.svcCtx.FileManagerRpc.UploadFile(l.ctx, &fileManagerClient.UploadFileRequest{
		Data:        data.Data,
		Filename:    data.Filename,
		ContentType: data.ContentType,
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
