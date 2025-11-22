package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/report/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/report/rpc/report"

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

// Attachments
func (l *UploadAttachmentLogic) UploadAttachment(in *report.UploadAttachmentRequest) (*report.UploadAttachmentResponse, error) {
	// todo: add your logic here and delete this line

	return &report.UploadAttachmentResponse{}, nil
}
