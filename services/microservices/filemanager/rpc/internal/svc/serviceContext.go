package svc

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/suleymanmyradov/growth-server/pkg/events/userdeletion"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/internal/cleanup"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/queue"
)

// ObjectStore is the subset of *minio.Client the service uses, defined as an
// interface so logic tests can substitute a fake.
type ObjectStore interface {
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
	PresignedGetObject(ctx context.Context, bucketName, objectName string, expiry time.Duration, reqParams url.Values) (*url.URL, error)
	BucketExists(ctx context.Context, bucketName string) (bool, error)
	MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error
}

type ServiceContext struct {
	Config config.Config
	Minio  ObjectStore
	// PresignMinio, when set, is a client pointed at the public origin so
	// presigned URLs are valid from the browser (SigV4 signatures are
	// host-bound). Nil when no public base URL is configured — presigning
	// then falls back to the internal client.
	PresignMinio ObjectStore
	// PresignPathPrefix is the path prefix (e.g. "/files") that the public
	// origin proxies to MinIO; presigned URLs get it prepended so Caddy
	// routes and strips it before MinIO validates the signature.
	PresignPathPrefix string
	// Queries is the file_objects registry (ownership + expiry). Nil when
	// Postgres is unconfigured — uploads/deletes then fall back to untracked
	// MinIO-only behavior and a warning is logged per call.
	Queries db.Querier
	// DeletionQ consumes user_deleted events and removes every object the
	// user owns. Nil when UserDeletion.Topic is empty.
	DeletionQ     queue.MessageQueue
	closeDeletion func()
	// Sweeper deletes objects past their expires_at (e.g. exports/ after
	// 24h). Nil when Postgres is unconfigured.
	Sweeper *cleanup.Worker
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

	// file_objects registry. Without it there is no ownership record, so
	// DeleteFile cannot authorize callers and account deletion cannot find a
	// user's objects — every environment that serves real users needs this.
	var queries db.Querier
	if c.Postgres.Datasource != "" {
		pool := postgres.MustOpenPool(c.Postgres.Datasource, c.Postgres.MaxOpenConns, c.Postgres.MaxIdleConns, c.Postgres.ConnMaxLifetime)
		queries = db.New(pool)
	} else {
		logx.Alert("filemanager: Postgres not configured — objects will be untracked (no ownership checks, no deletion fan-out, no expiry sweeps)")
	}

	var sweeper *cleanup.Worker
	if queries != nil {
		sweeper = cleanup.NewWorker(queries, mc, c.Cleanup.Interval, c.Cleanup.BatchSize)
	}

	// user_deleted consumer: delete every object the user owns, then its
	// registry rows. Each removal is idempotent so redelivery after a partial
	// pass is safe.
	deletionQ, closeDeletion, err := userdeletion.NewQueue(c.UserDeletion, userdeletion.Handler{
		Delete: func(ctx context.Context, userID uuid.UUID) error {
			return cleanup.DeleteUserObjects(ctx, queries, mc, userID)
		},
	})
	if err != nil {
		logx.Must(fmt.Errorf("failed to create user deletion queue: %w", err))
	}

	return &ServiceContext{
		Config:            c,
		Minio:             mc,
		PresignMinio:      presignMc,
		PresignPathPrefix: presignPrefix,
		Queries:           queries,
		DeletionQ:         deletionQ,
		closeDeletion:     closeDeletion,
		Sweeper:           sweeper,
	}
}

// StartBackground starts the user_deleted consumer and the expiry sweeper.
// Both are no-ops when disabled.
func (s *ServiceContext) StartBackground() {
	if s.DeletionQ != nil {
		go s.DeletionQ.Start()
	}
	if s.Sweeper != nil {
		s.Sweeper.Start()
	}
}

// StopBackground stops the consumer and sweeper and releases transport
// resources. Safe to call when disabled.
func (s *ServiceContext) StopBackground() {
	if s.DeletionQ != nil {
		s.DeletionQ.Stop()
	}
	if s.closeDeletion != nil {
		s.closeDeletion()
	}
	if s.Sweeper != nil {
		s.Sweeper.Stop()
	}
}
