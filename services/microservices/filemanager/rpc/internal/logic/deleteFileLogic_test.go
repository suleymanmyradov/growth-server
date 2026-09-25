package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/pb/filemanager"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func seedObject(t *testing.T, q *fakeQueries, bucket, key string, owner uuid.NullUUID) {
	t.Helper()
	err := q.CreateFileObject(context.Background(), db.CreateFileObjectParams{
		Bucket:      bucket,
		ObjectKey:   key,
		OwnerUserID: owner,
		Folder:      "avatars",
	})
	require.NoError(t, err)
}

func TestDeleteFile_OwnerCanDelete(t *testing.T) {
	q := newFakeQueries()
	objects := newFakeObjects()
	owner := uuid.New()
	key := "avatars/pic.png"
	seedObject(t, q, "test-bucket", key, uuid.NullUUID{UUID: owner, Valid: true})
	objects.put[regKey("test-bucket", key)] = []byte("data")

	l := NewDeleteFileLogic(context.Background(), testSvcCtx(q, objects))
	resp, err := l.DeleteFile(&filemanager.DeleteFileRequest{Key: key, UserId: owner.String()})
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Contains(t, objects.removed, "test-bucket/"+key)
	_, lookupErr := q.GetFileObject(context.Background(), "test-bucket", key)
	assert.Error(t, lookupErr) // registry row gone too
}

func TestDeleteFile_NotOwnerDenied(t *testing.T) {
	q := newFakeQueries()
	objects := newFakeObjects()
	key := "avatars/pic.png"
	seedObject(t, q, "test-bucket", key, uuid.NullUUID{UUID: uuid.New(), Valid: true})
	objects.put[regKey("test-bucket", key)] = []byte("data")

	l := NewDeleteFileLogic(context.Background(), testSvcCtx(q, objects))
	_, err := l.DeleteFile(&filemanager.DeleteFileRequest{Key: key, UserId: uuid.New().String()})
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Empty(t, objects.removed) // object untouched
}

func TestDeleteFile_UnknownKeyNotFound(t *testing.T) {
	q := newFakeQueries()
	l := NewDeleteFileLogic(context.Background(), testSvcCtx(q, newFakeObjects()))

	_, err := l.DeleteFile(&filemanager.DeleteFileRequest{Key: "avatars/missing.png", UserId: uuid.New().String()})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestDeleteFile_EmptyRequesterDenied(t *testing.T) {
	q := newFakeQueries()
	key := "avatars/pic.png"
	seedObject(t, q, "test-bucket", key, uuid.NullUUID{UUID: uuid.New(), Valid: true})

	l := NewDeleteFileLogic(context.Background(), testSvcCtx(q, newFakeObjects()))
	_, err := l.DeleteFile(&filemanager.DeleteFileRequest{Key: key})
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestDeleteFile_OwnerlessObjectDenied(t *testing.T) {
	q := newFakeQueries()
	key := "articles/cover.png"
	seedObject(t, q, "test-bucket", key, uuid.NullUUID{}) // admin upload, no owner

	l := NewDeleteFileLogic(context.Background(), testSvcCtx(q, newFakeObjects()))
	_, err := l.DeleteFile(&filemanager.DeleteFileRequest{Key: key, UserId: uuid.New().String()})
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestDeleteFile_NoRegistryLegacyAllow(t *testing.T) {
	objects := newFakeObjects()
	key := "avatars/pic.png"
	objects.put[regKey("test-bucket", key)] = []byte("data")

	// Queries nil → registry unconfigured → legacy untracked delete allowed.
	l := NewDeleteFileLogic(context.Background(), testSvcCtx(nil, objects))
	resp, err := l.DeleteFile(&filemanager.DeleteFileRequest{Key: key, UserId: uuid.New().String()})
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Contains(t, objects.removed, "test-bucket/"+key)
}

func TestDeleteFile_RemoveFails(t *testing.T) {
	q := newFakeQueries()
	objects := newFakeObjects()
	objects.removeErr = errors.New("minio down")
	owner := uuid.New()
	key := "avatars/pic.png"
	seedObject(t, q, "test-bucket", key, uuid.NullUUID{UUID: owner, Valid: true})

	l := NewDeleteFileLogic(context.Background(), testSvcCtx(q, objects))
	_, err := l.DeleteFile(&filemanager.DeleteFileRequest{Key: key, UserId: owner.String()})
	assert.Error(t, err)
	// Registry row survives so a retry can authorize again.
	_, lookupErr := q.GetFileObject(context.Background(), "test-bucket", key)
	assert.NoError(t, lookupErr)
}
