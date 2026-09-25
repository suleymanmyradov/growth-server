package logic

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/pb/filemanager"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

	// Authorization: the requester must own the object in the registry.
	// Objects with no owner (admin/service uploads) can never match a user
	// delete; objects with no registry row are not ours to delete.
	if l.svcCtx.Queries == nil {
		// Registry unavailable: keep the legacy untracked behavior rather than
		// breaking deletes outright. Callers must configure Postgres to get
		// ownership enforcement.
		logx.WithContext(ctx).Errorf("file_objects registry unavailable; deleting %s/%s without ownership check", bucket, in.Key)
	} else {
		obj, err := l.svcCtx.Queries.GetFileObject(ctx, bucket, in.Key)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "object not found")
		}
		if err != nil {
			return nil, fmt.Errorf("lookup object ownership: %w", err)
		}
		requester, parseErr := uuid.Parse(in.UserId)
		if in.UserId == "" || parseErr != nil || !obj.OwnerUserID.Valid || obj.OwnerUserID.UUID != requester {
			logx.WithContext(ctx).Errorf("delete denied: key %s not owned by requester", in.Key)
			return nil, status.Error(codes.PermissionDenied, "object not owned by requester")
		}
	}

	err := l.svcCtx.Minio.RemoveObject(ctx, bucket, in.Key, minio.RemoveObjectOptions{})
	if err != nil {
		logx.WithContext(ctx).Errorf("minio remove object failed: %v", err)
		return nil, fmt.Errorf("delete failed: %w", err)
	}

	// Object gone — drop the registry row. A failed row delete leaves a stale
	// row pointing at a missing object; the expiry sweeper or a later delete
	// reclaims it (RemoveObject is idempotent on absent keys).
	if l.svcCtx.Queries != nil {
		if err := l.svcCtx.Queries.DeleteFileObject(ctx, bucket, in.Key); err != nil {
			return nil, fmt.Errorf("delete registry row: %w", err)
		}
	}

	return &filemanager.DeleteFileResponse{
		Success: true,
	}, nil
}
