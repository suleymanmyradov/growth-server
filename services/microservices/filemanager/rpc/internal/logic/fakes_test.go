package logic

import (
	"context"
	"io"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/internal/svc"
)

// fakeQueries is an in-memory db.Querier for logic tests.
type fakeQueries struct {
	mu        sync.Mutex
	objects   map[string]db.FileObject
	created   []db.CreateFileObjectParams
	createErr error
	deleteErr error
}

func newFakeQueries() *fakeQueries {
	return &fakeQueries{objects: map[string]db.FileObject{}}
}

func regKey(bucket, key string) string { return bucket + "/" + key }

func (f *fakeQueries) CreateFileObject(_ context.Context, arg db.CreateFileObjectParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
	f.created = append(f.created, arg)
	f.objects[regKey(arg.Bucket, arg.ObjectKey)] = db.FileObject{
		Bucket:      arg.Bucket,
		ObjectKey:   arg.ObjectKey,
		OwnerUserID: arg.OwnerUserID,
		Folder:      arg.Folder,
		ContentType: arg.ContentType,
		SizeBytes:   arg.SizeBytes,
		ExpiresAt:   arg.ExpiresAt,
	}
	return nil
}

func (f *fakeQueries) DeleteFileObject(_ context.Context, bucket, objectKey string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.objects, regKey(bucket, objectKey))
	return nil
}

func (f *fakeQueries) GetFileObject(_ context.Context, bucket, objectKey string) (db.FileObject, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	obj, ok := f.objects[regKey(bucket, objectKey)]
	if !ok {
		return db.FileObject{}, pgx.ErrNoRows
	}
	return obj, nil
}

func (f *fakeQueries) ListExpiredFileObjects(context.Context, int32) ([]db.FileObject, error) {
	return nil, nil
}

func (f *fakeQueries) ListFileObjectsByOwner(_ context.Context, owner uuid.NullUUID) ([]db.FileObject, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []db.FileObject
	for _, obj := range f.objects {
		if obj.OwnerUserID == owner {
			out = append(out, obj)
		}
	}
	return out, nil
}

// fakeObjects is an in-memory svc.ObjectStore for logic tests.
type fakeObjects struct {
	mu        sync.Mutex
	put       map[string][]byte
	removed   []string
	putErr    error
	removeErr error
}

func newFakeObjects() *fakeObjects {
	return &fakeObjects{put: map[string][]byte{}}
}

func (f *fakeObjects) PutObject(_ context.Context, bucket, key string, reader io.Reader, _ int64, _ minio.PutObjectOptions) (minio.UploadInfo, error) {
	if f.putErr != nil {
		return minio.UploadInfo{}, f.putErr
	}
	data, _ := io.ReadAll(reader)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.put[regKey(bucket, key)] = data
	return minio.UploadInfo{Bucket: bucket, Key: key}, nil
}

func (f *fakeObjects) RemoveObject(_ context.Context, bucket, key string, _ minio.RemoveObjectOptions) error {
	if f.removeErr != nil {
		return f.removeErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.put, regKey(bucket, key))
	f.removed = append(f.removed, regKey(bucket, key))
	return nil
}

func (f *fakeObjects) PresignedGetObject(context.Context, string, string, time.Duration, url.Values) (*url.URL, error) {
	return &url.URL{Scheme: "https", Host: "files.example.com", Path: "/x"}, nil
}

func (f *fakeObjects) BucketExists(context.Context, string) (bool, error) { return true, nil }

func (f *fakeObjects) MakeBucket(context.Context, string, minio.MakeBucketOptions) error { return nil }

// testSvcCtx builds a ServiceContext with the given registry/object fakes and
// a default bucket so logic code paths resolve the bucket name.
func testSvcCtx(queries db.Querier, objects svc.ObjectStore) *svc.ServiceContext {
	var c config.Config
	c.MinIO.DefaultBucket = "test-bucket"
	return &svc.ServiceContext{
		Config:  c,
		Minio:   objects,
		Queries: queries,
	}
}
