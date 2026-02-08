package reportlogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
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
	l.Logger.Infof("Uploading attachment for report %s", in.ReportId)

	return &client.UploadAttachmentResponse{
		AttachmentUrl: "",
	}, nil
}
