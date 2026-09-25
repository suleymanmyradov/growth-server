package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/pb/filemanager"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestUploadFile_RecordsOwnership(t *testing.T) {
	q := newFakeQueries()
	objects := newFakeObjects()
	owner := uuid.New()

	l := NewUploadFileLogic(context.Background(), testSvcCtx(q, objects))
	resp, err := l.UploadFile(&filemanager.UploadFileRequest{
		Data:     pngBytes,
		Filename: "pic.png",
		Folder:   "avatars",
		UserId:   owner.String(),
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Key)

	require.Len(t, q.created, 1)
	row := q.created[0]
	assert.Equal(t, resp.Key, row.ObjectKey)
	assert.Equal(t, "test-bucket", row.Bucket)
	assert.True(t, row.OwnerUserID.Valid)
	assert.Equal(t, owner, row.OwnerUserID.UUID)
	assert.Equal(t, "avatars", row.Folder)
	assert.Equal(t, "image/png", row.ContentType)
	assert.Equal(t, int64(len(pngBytes)), row.SizeBytes)
	assert.False(t, row.ExpiresAt.Valid) // avatars never expire
}

func TestUploadFile_ExportGetsExpiry(t *testing.T) {
	q := newFakeQueries()
	l := NewUploadFileLogic(context.Background(), testSvcCtx(q, newFakeObjects()))

	before := time.Now()
	_, err := l.UploadFile(&filemanager.UploadFileRequest{
		Data:     jsonBytes,
		Filename: "export.json",
		Folder:   "exports",
		UserId:   uuid.New().String(),
	})
	require.NoError(t, err)

	require.Len(t, q.created, 1)
	expiresAt := q.created[0].ExpiresAt
	require.True(t, expiresAt.Valid, "exports must carry an expiry")
	assert.True(t, expiresAt.Time.After(before.Add(23*time.Hour)))
	assert.True(t, expiresAt.Time.Before(before.Add(25*time.Hour)))
}

func TestUploadFile_InvalidUserIDRejected(t *testing.T) {
	q := newFakeQueries()
	objects := newFakeObjects()
	l := NewUploadFileLogic(context.Background(), testSvcCtx(q, objects))

	_, err := l.UploadFile(&filemanager.UploadFileRequest{
		Data:     pngBytes,
		Filename: "pic.png",
		Folder:   "avatars",
		UserId:   "not-a-uuid",
	})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Empty(t, objects.put)
	assert.Empty(t, q.created)
}

func TestUploadFile_RegistryFailureRemovesOrphan(t *testing.T) {
	q := newFakeQueries()
	q.createErr = errors.New("db down")
	objects := newFakeObjects()
	l := NewUploadFileLogic(context.Background(), testSvcCtx(q, objects))

	_, err := l.UploadFile(&filemanager.UploadFileRequest{
		Data:     pngBytes,
		Filename: "pic.png",
		Folder:   "avatars",
		UserId:   uuid.New().String(),
	})
	require.Error(t, err)
	// The stored object must not be left as an untracked orphan.
	assert.Empty(t, objects.put)
	assert.Len(t, objects.removed, 1)
}

func TestUploadFile_NoRegistryUploadsUntracked(t *testing.T) {
	objects := newFakeObjects()
	l := NewUploadFileLogic(context.Background(), testSvcCtx(nil, objects))

	resp, err := l.UploadFile(&filemanager.UploadFileRequest{
		Data:     pngBytes,
		Filename: "pic.png",
		Folder:   "avatars",
		UserId:   uuid.New().String(),
	})
	require.NoError(t, err)
	assert.Contains(t, objects.put, "test-bucket/"+resp.Key)
}

func TestUploadFile_AdminUploadHasNoOwner(t *testing.T) {
	q := newFakeQueries()
	l := NewUploadFileLogic(context.Background(), testSvcCtx(q, newFakeObjects()))

	_, err := l.UploadFile(&filemanager.UploadFileRequest{
		Data:     pngBytes,
		Filename: "cover.png",
		Folder:   "articles",
		// no UserId — admin/service upload
	})
	require.NoError(t, err)
	require.Len(t, q.created, 1)
	assert.False(t, q.created[0].OwnerUserID.Valid)
}
