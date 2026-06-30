package svc

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/internal/config"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceContext struct {
	Config config.Config
	Minio  *minio.Client
}

func NewServiceContext(c config.Config) *ServiceContext {
	mc, err := minio.New(c.MinIO.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(c.MinIO.AccessKey, c.MinIO.SecretKey, ""),
		Secure: c.MinIO.UseSSL,
		Region: c.MinIO.Region,
	})
	if err != nil {
		logx.Must(fmt.Errorf("init minio client: %w", err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := mc.BucketExists(ctx, c.MinIO.DefaultBucket)
	if err != nil {
		logx.Must(fmt.Errorf("check minio bucket: %w", err))
	}
	if !exists {
		err = mc.MakeBucket(ctx, c.MinIO.DefaultBucket, minio.MakeBucketOptions{Region: c.MinIO.Region})
		if err != nil {
			logx.Must(fmt.Errorf("create minio bucket: %w", err))
		}
		logx.Infof("created minio bucket: %s", c.MinIO.DefaultBucket)
	}

	return &ServiceContext{
		Config: c,
		Minio:  mc,
	}
}
