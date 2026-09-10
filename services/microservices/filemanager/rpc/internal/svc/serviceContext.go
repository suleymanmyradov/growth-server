package svc

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/internal/config"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceContext struct {
	Config config.Config
	Minio  *minio.Client
	// PresignMinio, when set, is a client pointed at the public origin so
	// presigned URLs are valid from the browser (SigV4 signatures are
	// host-bound). Nil when no public base URL is configured — presigning
	// then falls back to the internal client.
	PresignMinio *minio.Client
	// PresignPathPrefix is the path prefix (e.g. "/files") that the public
	// origin proxies to MinIO; presigned URLs get it prepended so Caddy
	// routes and strips it before MinIO validates the signature.
	PresignPathPrefix string
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

	// Presigned URLs must be valid from the browser-reachable origin. The
	// internal client signs for the cluster-internal endpoint, which clients
	// cannot resolve, so a second client is built against the public host
	// parsed from PublicBaseUrl (e.g. https://api.example.com/files).
	var presignMc *minio.Client
	presignPrefix := ""
	if publicBase := c.MinIO.PublicBaseUrl; publicBase != "" {
		if u, err := url.Parse(publicBase); err == nil && u.Host != "" {
			presignClient, err := minio.New(u.Host, &minio.Options{
				Creds:  credentials.NewStaticV4(c.MinIO.AccessKey, c.MinIO.SecretKey, ""),
				Secure: u.Scheme == "https",
				Region: c.MinIO.Region,
			})
			if err != nil {
				logx.Must(fmt.Errorf("init minio presign client: %w", err))
			}
			presignMc = presignClient
			presignPrefix = u.Path
		}
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
		Config:            c,
		Minio:             mc,
		PresignMinio:      presignMc,
		PresignPathPrefix: presignPrefix,
	}
}
