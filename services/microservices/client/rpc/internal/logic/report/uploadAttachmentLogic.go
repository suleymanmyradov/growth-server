package reportlogic

import (
	"context"
	"fmt"
	"mime"
	"path/filepath"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/fileManagerClient"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UploadAttachmentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUploadAttachmentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UploadAttachmentLogic {
	return &UploadAttachmentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UploadAttachmentLogic) UploadAttachment(in *client.UploadAttachmentRequest) (*client.UploadAttachmentResponse, error) {
	l.Infof("Uploading attachment for report %s", in.ReportId)

	ext := filepath.Ext(in.Filename)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	uploadResp, err := l.svcCtx.FileManagerRpc.UploadFile(l.ctx, &fileManagerClient.UploadFileRequest{
		Data:        in.Data,
		Filename:    in.Filename,
		ContentType: contentType,
		Folder:      fmt.Sprintf("attachments/%s", in.ReportId),
	})
	if err != nil {
		l.Errorf("upload attachment to filemanager failed: %v", err)
		return nil, status.Error(codes.Internal, "attachment upload failed")
	}

	return &client.UploadAttachmentResponse{
		AttachmentUrl: uploadResp.Url,
	}, nil
}
