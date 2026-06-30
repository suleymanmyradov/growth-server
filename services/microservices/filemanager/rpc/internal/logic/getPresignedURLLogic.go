package logic

import (
	"context"
	"fmt"
	"time"

	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/pb/filemanager"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type GetPresignedURLLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPresignedURLLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPresignedURLLogic {
	return &GetPresignedURLLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetPresignedURLLogic) GetPresignedURL(in *filemanager.GetPresignedURLRequest) (*filemanager.GetPresignedURLResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetPresignedURLLogic.GetPresignedURL")
	defer span.End()

	bucket := in.Bucket
	if bucket == "" {
		bucket = l.svcCtx.Config.MinIO.DefaultBucket
	}

	expiry := time.Duration(in.ExpirySeconds) * time.Second
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}

	reqParams := make(map[string][]string)
	presignedURL, err := l.svcCtx.Minio.PresignedGetObject(ctx, bucket, in.Key, expiry, reqParams)
	if err != nil {
		logx.WithContext(ctx).Errorf("minio presigned url failed: %v", err)
		return nil, fmt.Errorf("presigned url failed: %w", err)
	}

	return &filemanager.GetPresignedURLResponse{
		Url: presignedURL.String(),
	}, nil
}
