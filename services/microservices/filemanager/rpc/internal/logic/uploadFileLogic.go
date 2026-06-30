package logic

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/pb/filemanager"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

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
	if l.svcCtx.Config.MinIO.UseSSL {
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
