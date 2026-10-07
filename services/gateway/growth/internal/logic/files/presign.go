package files

import (
	"context"
	"net/url"
	"strings"

	fileManagerClient "github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/fileManagerClient"

	"github.com/zeromicro/go-zero/core/logx"
)

// presignExpirySeconds is how long a minted avatar URL stays valid. SigV4
// caps presigned URLs at 7 days; a client holding a cached profile longer
// than that sees a broken avatar until the next profile read refreshes it.
const presignExpirySeconds = 7 * 24 * 60 * 60

// objectURL describes a filemanager-served object URL:
// <origin>/files/<bucket>/<key>.
type objectURL struct {
	Bucket string
	Key    string
}

// parseObjectURL recognizes URLs served through the /files proxy
// (path-style /files/<bucket>/<key>). It returns ok=false for anything else
// — external avatar hosts (e.g. OAuth provider images), relative paths,
// malformed input — which callers pass through unchanged.
func parseObjectURL(raw string) (objectURL, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Path == "" {
		return objectURL{}, false
	}
	parts := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 3)
	if len(parts) != 3 || parts[0] != "files" || parts[1] == "" || parts[2] == "" {
		return objectURL{}, false
	}
	return objectURL{Bucket: parts[1], Key: parts[2]}, true
}

// ResolveAvatarURL rewrites a stored avatar URL into a presigned URL that
// the browser can fetch. Avatars live under a private prefix, so the stored
// canonical URL is not fetchable directly — every read mints a fresh
// presigned URL via the filemanager (a local HMAC computation, no MinIO
// round trip). Non-filemanager URLs (e.g. OAuth provider avatars) pass
// through unchanged, and on presign failure the stored URL is returned so
// auth/profile responses never fail over an image.
func ResolveAvatarURL(ctx context.Context, fm fileManagerClient.FileManager, raw string) string {
	if raw == "" || fm == nil {
		return raw
	}
	obj, ok := parseObjectURL(raw)
	if !ok || !strings.HasPrefix(obj.Key, "avatars/") {
		return raw
	}
	resp, err := fm.GetPresignedURL(ctx, &fileManagerClient.GetPresignedURLRequest{
		Bucket:        obj.Bucket,
		Key:           obj.Key,
		ExpirySeconds: presignExpirySeconds,
	})
	if err != nil || resp.GetUrl() == "" {
		logx.WithContext(ctx).Errorf("presign avatar url failed, serving stored url: %v", err)
		return raw
	}
	return resp.GetUrl()
}

// CanonicalFileURL strips presign query parameters from a filemanager
// object URL. Clients echo avatarUrl back on profile updates (both the web
// form and the mobile app persist what the API returned), so an expiring
// presigned URL must never be stored — normalize to the canonical unsigned
// form. Non-filemanager URLs pass through unchanged.
func CanonicalFileURL(raw string) string {
	if _, ok := parseObjectURL(raw); !ok {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}
