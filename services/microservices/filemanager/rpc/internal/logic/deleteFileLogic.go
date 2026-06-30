package logic

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/pb/filemanager"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type DeleteFileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteFileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteFileLogic {
	return &DeleteFileLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteFileLogic) DeleteFile(in *filemanager.DeleteFileRequest) (*filemanager.DeleteFileResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "DeleteFileLogic.DeleteFile")
	defer span.End()

	bucket := in.Bucket
	if bucket == "" {
		bucket = l.svcCtx.Config.MinIO.DefaultBucket
	}

	err := l.svcCtx.Minio.RemoveObject(ctx, bucket, in.Key, minio.RemoveObjectOptions{})
	if err != nil {
		logx.WithContext(ctx).Errorf("minio remove object failed: %v", err)
		return nil, fmt.Errorf("delete failed: %w", err)
	}

	return &filemanager.DeleteFileResponse{
		Success: true,
	}, nil
}
