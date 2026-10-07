package files

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fileManagerClient "github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/fileManagerClient"
	"google.golang.org/grpc"
)

type fakeFileManager struct {
	uploadErr   error
	presignErr  error
	presignResp *fileManagerClient.GetPresignedURLResponse

	uploadCalls   int
	presignCalls  int
	lastPresignIn *fileManagerClient.GetPresignedURLRequest
}

func (f *fakeFileManager) UploadFile(ctx context.Context, in *fileManagerClient.UploadFileRequest, opts ...grpc.CallOption) (*fileManagerClient.UploadFileResponse, error) {
	f.uploadCalls++
	return nil, f.uploadErr
}

func (f *fakeFileManager) GetPresignedURL(ctx context.Context, in *fileManagerClient.GetPresignedURLRequest, opts ...grpc.CallOption) (*fileManagerClient.GetPresignedURLResponse, error) {
	f.presignCalls++
	f.lastPresignIn = in
	if f.presignErr != nil {
		return nil, f.presignErr
	}
	return f.presignResp, nil
}

func (f *fakeFileManager) DeleteFile(ctx context.Context, in *fileManagerClient.DeleteFileRequest, opts ...grpc.CallOption) (*fileManagerClient.DeleteFileResponse, error) {
	return nil, nil
}

func TestResolveAvatarURL(t *testing.T) {
	const presigned = "https://api.example.com/files/growthmind/avatars/abc.png?X-Amz-Signature=sig"
	fm := &fakeFileManager{presignResp: &fileManagerClient.GetPresignedURLResponse{Url: presigned}}
	ctx := context.Background()

	t.Run("presigns stored avatar urls", func(t *testing.T) {
		got := ResolveAvatarURL(ctx, fm, "https://api.example.com/files/growthmind/avatars/abc.png")
		require.Equal(t, presigned, got)
		require.Equal(t, 1, fm.presignCalls)
		require.Equal(t, "growthmind", fm.lastPresignIn.Bucket)
		require.Equal(t, "avatars/abc.png", fm.lastPresignIn.Key)
		assert.Equal(t, int32(7*24*60*60), fm.lastPresignIn.ExpirySeconds)
	})

	t.Run("passes through non-filemanager urls", func(t *testing.T) {
		const external = "https://lh3.googleusercontent.com/a/photo"
		assert.Equal(t, external, ResolveAvatarURL(ctx, fm, external))
		assert.Equal(t, "", ResolveAvatarURL(ctx, fm, ""))
		assert.Equal(t, 1, fm.presignCalls, "no extra presign calls")
	})

	t.Run("passes through other folders untouched", func(t *testing.T) {
		const article = "https://api.example.com/files/growthmind/articles/abc.webp"
		assert.Equal(t, article, ResolveAvatarURL(ctx, fm, article))
		const export = "https://api.example.com/files/growthmind/exports/x.json"
		assert.Equal(t, export, ResolveAvatarURL(ctx, fm, export))
		assert.Equal(t, 1, fm.presignCalls, "no extra presign calls")
	})

	t.Run("falls back to stored url on presign failure", func(t *testing.T) {
		failing := &fakeFileManager{presignErr: errors.New("filemanager down")}
		const stored = "https://api.example.com/files/growthmind/avatars/abc.png"
		assert.Equal(t, stored, ResolveAvatarURL(ctx, failing, stored))
	})

	t.Run("falls back when presign returns empty url", func(t *testing.T) {
		empty := &fakeFileManager{presignResp: &fileManagerClient.GetPresignedURLResponse{}}
		const stored = "https://api.example.com/files/growthmind/avatars/abc.png"
		assert.Equal(t, stored, ResolveAvatarURL(ctx, empty, stored))
	})

	t.Run("nil client returns raw url", func(t *testing.T) {
		const stored = "https://api.example.com/files/growthmind/avatars/abc.png"
		assert.Equal(t, stored, ResolveAvatarURL(ctx, nil, stored))
	})
}

func TestCanonicalFileURL(t *testing.T) {
	t.Run("strips presign query from filemanager urls", func(t *testing.T) {
		const presigned = "https://api.example.com/files/growthmind/avatars/abc.png?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Signature=sig&X-Amz-Expires=604800"
		assert.Equal(t, "https://api.example.com/files/growthmind/avatars/abc.png", CanonicalFileURL(presigned))
	})

	t.Run("leaves external urls untouched", func(t *testing.T) {
		const external = "https://lh3.googleusercontent.com/a/photo?size=200"
		assert.Equal(t, external, CanonicalFileURL(external))
	})

	t.Run("leaves canonical urls untouched", func(t *testing.T) {
		const canonical = "https://api.example.com/files/growthmind/avatars/abc.png"
		assert.Equal(t, canonical, CanonicalFileURL(canonical))
	})

	t.Run("handles malformed input", func(t *testing.T) {
		assert.Equal(t, "", CanonicalFileURL(""))
		assert.Equal(t, "not a url", CanonicalFileURL("not a url"))
	})
}

func TestParseObjectURL(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		want   objectURL
		wantOK bool
	}{
		{"avatar", "https://api.example.com/files/growthmind/avatars/abc.png", objectURL{Bucket: "growthmind", Key: "avatars/abc.png"}, true},
		{"presigned avatar", "https://api.example.com/files/growthmind/avatars/abc.png?X-Amz-Signature=sig", objectURL{Bucket: "growthmind", Key: "avatars/abc.png"}, true},
		{"nested key", "http://localhost:9000/files/growthmind/exports/deep/dir/x.json", objectURL{Bucket: "growthmind", Key: "exports/deep/dir/x.json"}, true},
		{"bucket root", "https://api.example.com/files/growthmind/", objectURL{}, false},
		{"missing key", "https://api.example.com/files/growthmind", objectURL{}, false},
		{"wrong prefix", "https://api.example.com/storage/growthmind/avatars/abc.png", objectURL{}, false},
		{"empty", "", objectURL{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseObjectURL(tt.raw)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
